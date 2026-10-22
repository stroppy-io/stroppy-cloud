package run

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Subnet allocation: a Yandex run joins its profile's network and gets one
// subnet per zone it places hosts in, cut out of the profile's address
// pool when the queue admits it (under the tenant lock; profiles are
// tenant-scoped, so no other admission sees the same pool meanwhile). The
// blocks go into the stored RunSpec (network.subnets): the pipeline creates
// exactly those and nothing else, and held blocks are read back from runs.

// subnetCleanup is how long a finished run's blocks stay held: Graphene
// deletes the run's subnets only after the run is terminal, and a block
// reused before that collides in the cloud.
const subnetCleanup = time.Hour

// errPoolFull is a pool without a free block for the run right now.
var errPoolFull = errors.New("no free block in the address pool")

// subnetPool is the allocation state of one profile: the blocks held by
// its runs and the cursor, the address after the latest allocation.
// Blocks go round-robin from the cursor, so a freed block is reused as late
// as possible.
type subnetPool struct {
	held   []netip.Prefix
	cursor uint32
	// started is set once some run allocated: the cursor is real.
	started bool
}

// newSubnetPool reads the held blocks and the cursor from the profile's
// runs, in admission order.
func newSubnetPool(runs []NetworkRun) *subnetPool {
	p := &subnetPool{}
	for _, r := range runs {
		var spec pipelinespec.Run
		if json.Unmarshal(r.Spec, &spec) != nil {
			continue
		}
		var last netip.Prefix
		for _, sub := range spec.Network.Subnets {
			prefix, err := netip.ParsePrefix(sub.CIDR)
			if err != nil || !prefix.Addr().Is4() {
				continue
			}
			prefix = prefix.Masked()
			if r.Holds {
				p.held = append(p.held, prefix)
			}
			last = prefix
		}
		if last.IsValid() {
			p.cursor, p.started = end(last), true
		}
	}
	return p
}

// allocate gives every zone a block of the pool sized to its hosts
// (YandexSubnetBits), first fit from the cursor with alignment, wrapping
// once around the pool. All zones or none: a run that does not fit takes
// nothing and the pool stays as it was.
func (p *subnetPool) allocate(poolCIDR string, zones []pipelinespec.ZoneDemand) ([]pipelinespec.Subnet, error) {
	pool, err := netip.ParsePrefix(poolCIDR)
	if err != nil || !pool.Addr().Is4() {
		return nil, fmt.Errorf("address pool %q is not an IPv4 range", poolCIDR)
	}
	pool = pool.Masked()
	if pool.Bits() < 8 {
		return nil, fmt.Errorf("address pool %s is wider than /8", pool)
	}
	first, size := start(pool), blockSize(pool.Bits())
	cursor := p.cursor
	if !p.started || cursor < first || cursor-first >= size {
		cursor = first
	}
	held := append([]netip.Prefix(nil), p.held...)
	out := make([]pipelinespec.Subnet, 0, len(zones))
	for _, zone := range zones {
		bits, err := pipelinespec.YandexSubnetBits(zone.Hosts)
		if err != nil {
			return nil, err
		}
		block, ok := firstFit(first, size, bits, cursor, held, pool.Bits())
		if !ok {
			return nil, errPoolFull
		}
		held = append(held, block)
		cursor = end(block)
		if cursor-first >= size {
			cursor = first
		}
		out = append(out, pipelinespec.Subnet{Zone: zone.Zone, CIDR: block.String()})
	}
	p.held, p.cursor, p.started = held, cursor, true
	return out, nil
}

// firstFit is the first aligned /bits block of the pool at or after the
// cursor (wrapping to the pool start) that overlaps no held block.
func firstFit(first, size uint32, bits int, cursor uint32, held []netip.Prefix, poolBits int) (netip.Prefix, bool) {
	if bits < poolBits {
		return netip.Prefix{}, false
	}
	step := blockSize(bits)
	offset := (cursor - first + step - 1) / step * step // align up inside the pool
	slots := size / step
	for i := uint32(0); i < slots; i++ {
		at := (offset/step + i) % slots * step
		block := prefixAt(first+at, bits)
		if !overlapsAny(block, held) {
			return block, true
		}
	}
	return netip.Prefix{}, false
}

func overlapsAny(block netip.Prefix, held []netip.Prefix) bool {
	for _, h := range held {
		if block.Overlaps(h) {
			return true
		}
	}
	return false
}

// blockSize is the address count of a /bits block; 0 stands for 2^32.
func blockSize(bits int) uint32 { return uint32(1) << (32 - bits) }

func start(p netip.Prefix) uint32 {
	a := p.Masked().Addr().As4()
	return binary.BigEndian.Uint32(a[:])
}

// end is the address after the block.
func end(p netip.Prefix) uint32 { return start(p) + blockSize(p.Bits()) }

func prefixAt(addr uint32, bits int) netip.Prefix {
	var a [4]byte
	binary.BigEndian.PutUint32(a[:], addr)
	return netip.PrefixFrom(netip.AddrFrom4(a), bits)
}
