package provision

import (
	"fmt"
	"net/netip"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type yandexSubnet struct{ Zone, Name, CIDR string }

// yandexSubnets pairs the run's zones (spec.YandexZones, the set the server
// allocated for) with the blocks the server put into network.subnets. A
// single-zone run keeps the plain subnet name. Spec order is deterministic
// during workflow replay; map iteration never determines resource names.
// The run never invents a range: the profile network is shared, so a block
// the server did not allocate could collide with another run.
func yandexSubnets(run spec.Run, name string) ([]yandexSubnet, error) {
	zones, err := spec.YandexZones(run)
	if err != nil {
		return nil, fmt.Errorf("provision/yandex: %w", err)
	}
	blocks := make(map[string]string, len(run.Network.Subnets))
	for _, sub := range run.Network.Subnets {
		prefix, err := netip.ParsePrefix(sub.CIDR)
		if err != nil || !prefix.Addr().Is4() || prefix.Masked() != prefix {
			return nil, fmt.Errorf("provision/yandex: invalid IPv4 subnet %q of zone %s", sub.CIDR, sub.Zone)
		}
		if prefix.Bits() < spec.YandexSubnetMinBits || prefix.Bits() > spec.YandexSubnetMaxBits {
			return nil, fmt.Errorf("provision/yandex: subnet %s of zone %s is outside /%d../%d", sub.CIDR, sub.Zone, spec.YandexSubnetMinBits, spec.YandexSubnetMaxBits)
		}
		blocks[sub.Zone] = prefix.String()
	}
	out := make([]yandexSubnet, 0, len(zones))
	for _, zone := range zones {
		cidr, ok := blocks[zone.Zone]
		if !ok {
			return nil, fmt.Errorf("provision/yandex: no subnet allocated for zone %s; the server allocates network.subnets when it admits the run", zone.Zone)
		}
		subnetName := name
		if len(zones) > 1 {
			subnetName += "-" + zone.Zone
		}
		out = append(out, yandexSubnet{Zone: zone.Zone, Name: subnetName, CIDR: cidr})
	}
	for i, sub := range out {
		for _, other := range out[i+1:] {
			if netip.MustParsePrefix(sub.CIDR).Overlaps(netip.MustParsePrefix(other.CIDR)) {
				return nil, fmt.Errorf("provision/yandex: subnets %s and %s overlap", sub.CIDR, other.CIDR)
			}
		}
	}
	return out, nil
}
