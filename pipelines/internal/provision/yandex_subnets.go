package provision

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net/netip"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type yandexSubnet struct{ Zone, Name, CIDR string }

// Split the run's IPv4 range into disjoint zonal subnets. A single-zone
// run keeps its original CIDR and resource name. Spec order is deterministic
// during workflow replay; map iteration never determines resource names.
func yandexSubnets(run spec.Run, fallback, name string) ([]yandexSubnet, error) {
	var zones []string
	seen := map[string]bool{}
	for _, m := range run.Machines {
		zone := m.Location
		if zone == "" {
			zone = fallback
		}
		if zone == "" {
			return nil, fmt.Errorf("provision/yandex: machine %s has no zone", m.Name)
		}
		if !seen[zone] {
			zones = append(zones, zone)
			seen[zone] = true
		}
	}
	if run.ManagedYDB != nil && run.ManagedYDB.Type == "dedicated" {
		for _, zone := range run.ManagedYDB.Zones {
			if !seen[zone] {
				zones = append(zones, zone)
				seen[zone] = true
			}
		}
	}
	if len(zones) == 0 {
		zones = append(zones, fallback)
	}
	prefix, err := netip.ParsePrefix(intraCIDR(run))
	if err != nil || !prefix.Addr().Is4() {
		return nil, fmt.Errorf("provision/yandex: invalid IPv4 network %q", intraCIDR(run))
	}
	additionalZones := len(zones) - 1
	if additionalZones < 0 {
		return nil, fmt.Errorf("provision/yandex: no availability zones")
	}
	width := prefix.Bits() + bits.Len(uint(additionalZones))
	if width > 28 {
		return nil, fmt.Errorf("provision/yandex: network %s is too small for %d zonal subnets (minimum /28 each)", prefix, len(zones))
	}
	addr := prefix.Masked().Addr().As4()
	base := uint64(binary.BigEndian.Uint32(addr[:]))
	out := make([]yandexSubnet, 0, len(zones))
	for i, zone := range zones {
		subnetName := name
		if len(zones) > 1 {
			subnetName += "-" + zone
		}
		address := base + uint64(i)*(uint64(1)<<uint(32-width))
		if address > 0xffffffff {
			return nil, fmt.Errorf("subnet address exceeds IPv4 range")
		}
		binary.BigEndian.PutUint32(addr[:], uint32(address))
		out = append(out, yandexSubnet{Zone: zone, Name: subnetName, CIDR: netip.PrefixFrom(netip.AddrFrom4(addr), width).String()})
	}
	return out, nil
}
