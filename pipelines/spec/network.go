package spec

import (
	"encoding/json"
	"fmt"
)

// The Yandex network model: one VPC network per provider profile, made by
// stroppy-provider-config (or named by the user), and per run only a
// security group plus one subnet per zone, cut by the server out of the
// profile's address pool. The cloud quota on networks is the scarce one;
// subnets and groups have room.
//
// doc: https://yandex.cloud/en/docs/vpc/concepts/limits

// Yandex network kinds of provider.yandex.settings@1.
const (
	YandexNetworkCreate   = "create"
	YandexNetworkExisting = "existing"
)

// DefaultYandexPool is the address pool of a created profile network.
const DefaultYandexPool = "10.128.0.0/16"

// YandexNetwork is the network part of a baked provider.yandex.settings@1.
type YandexNetwork struct {
	Kind string
	// PoolCIDR is the range run subnets are allocated from.
	PoolCIDR string
	// NetworkID and RouteTableID are the user's own objects (existing).
	NetworkID    string
	RouteTableID string
	// PublicIPs is the profile default of a machine's public address.
	PublicIPs bool
}

// YandexNetworkOf reads the network of baked Yandex settings. A missing
// network block is a created one; a missing public_ips is the schema
// default (true).
func YandexNetworkOf(settings json.RawMessage) (YandexNetwork, error) {
	var st struct {
		Network struct {
			Kind         string `json:"kind"`
			PoolCIDR     string `json:"pool_cidr"`
			NetworkID    string `json:"network_id"`
			RouteTableID string `json:"route_table_id"`
		} `json:"network"`
		PublicIPs *bool `json:"public_ips"`
	}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &st); err != nil {
			return YandexNetwork{}, fmt.Errorf("yandex settings: %w", err)
		}
	}
	n := YandexNetwork{
		Kind: st.Network.Kind, PoolCIDR: st.Network.PoolCIDR,
		NetworkID: st.Network.NetworkID, RouteTableID: st.Network.RouteTableID, PublicIPs: true,
	}
	if n.Kind == "" {
		n.Kind = YandexNetworkCreate
	}
	if st.PublicIPs != nil {
		n.PublicIPs = *st.PublicIPs
	}
	switch n.Kind {
	case YandexNetworkCreate:
	case YandexNetworkExisting:
		if n.NetworkID == "" || n.PoolCIDR == "" {
			return YandexNetwork{}, fmt.Errorf("yandex settings: an existing network needs network_id and pool_cidr")
		}
	default:
		return YandexNetwork{}, fmt.Errorf("yandex settings: unknown network kind %q", n.Kind)
	}
	return n, nil
}

// Created reports a network the profile pipeline creates and deletes.
func (n YandexNetwork) Created() bool { return n.Kind == YandexNetworkCreate }

// Pool is the range run subnets are allocated from. A profile stored
// before the pool (with the retired subnet_cidr) gets the default.
func (n YandexNetwork) Pool() string {
	if n.PoolCIDR != "" {
		return n.PoolCIDR
	}
	return DefaultYandexPool
}

// SharedEgress reports a profile network with its own route out for
// machines without a public address: the NAT gateway a created network
// gets when public_ips is off, or the user's route table. A run that needs
// private egress on a network without one declares its own gateway.
func (n YandexNetwork) SharedEgress() bool {
	if n.Created() {
		return !n.PublicIPs
	}
	return n.RouteTableID != ""
}

// ProfileNetworkName is the Crossplane object (and cloud name) of a
// created profile network; providerConfig is spec.ProviderConfigName.
// The suffixes keep the longest name (52 + 7) inside the 63 characters a
// Yandex Cloud resource name allows.
//
// doc: https://yandex.cloud/en/docs/vpc/concepts/network
func ProfileNetworkName(providerConfig string) string { return providerConfig + "-net" }

// ProfileGatewayName is the shared NAT gateway of a created profile network.
func ProfileGatewayName(providerConfig string) string { return providerConfig + "-nat" }

// ProfileRouteTableName is the route table sending 0.0.0.0/0 to the shared
// NAT gateway.
func ProfileRouteTableName(providerConfig string) string { return providerConfig + "-routes" }

// ZoneDemand is one zone a Yandex run places hosts in and how many
// addresses of its subnet those hosts take.
type ZoneDemand struct {
	Zone  string
	Hosts int
}

// YandexZones is the zones of a Yandex run in creation order — machines in
// spec order (no location: the profile zone), then the zones of a dedicated
// managed YDB — with the hosts each zone's subnet holds. The server sizes
// and allocates one subnet per entry; the pipeline creates exactly these.
func YandexZones(run Run) ([]ZoneDemand, error) {
	var st struct {
		Zone string `json:"zone"`
	}
	if len(run.Provider.Settings) > 0 {
		if err := json.Unmarshal(run.Provider.Settings, &st); err != nil {
			return nil, fmt.Errorf("yandex settings: %w", err)
		}
	}
	var out []ZoneDemand
	index := map[string]int{}
	add := func(zone string, hosts int) {
		i, ok := index[zone]
		if !ok {
			i = len(out)
			index[zone] = i
			out = append(out, ZoneDemand{Zone: zone})
		}
		out[i].Hosts += hosts
	}
	for _, m := range run.Machines {
		zone := m.Location
		if zone == "" {
			zone = st.Zone
		}
		if zone == "" {
			return nil, fmt.Errorf("machine %s has no zone", m.Name)
		}
		add(zone, 1)
	}
	// Dedicated YDB nodes take addresses of the run subnets of their zones.
	if y := run.ManagedYDB; y != nil && y.Type == "dedicated" && len(y.Zones) > 0 {
		perZone := max(1, (y.NodeCount+len(y.Zones)-1)/len(y.Zones))
		for _, zone := range y.Zones {
			add(zone, perZone)
		}
	}
	if len(out) == 0 {
		if st.Zone == "" {
			return nil, fmt.Errorf("run has no availability zone")
		}
		add(st.Zone, 0)
	}
	return out, nil
}

// yandexSubnetReserved is what a subnet takes before any host: the
// gateway (first address) and DNS (second) that Yandex Cloud reserves,
// plus the network and broadcast addresses of the block.
//
// doc: https://yandex.cloud/en/docs/vpc/concepts/network#subnet
const yandexSubnetReserved = 4

// Run subnet prefixes: /28 is the smallest subnet Yandex Cloud creates; a
// run block never exceeds /24.
const (
	YandexSubnetMinBits = 24
	YandexSubnetMaxBits = 28
)

// YandexSubnetBits is the prefix of a run subnet holding hosts: the
// smallest power-of-two block with room for the hosts and the reserved
// addresses twice over, between /28 and /24. Without the headroom a /24
// still holds up to 252 hosts; more do not fit any run block.
func YandexSubnetBits(hosts int) (int, error) {
	need := hosts + yandexSubnetReserved
	for bits := YandexSubnetMaxBits; bits >= YandexSubnetMinBits; bits-- {
		if 1<<(32-bits) >= 2*need {
			return bits, nil
		}
	}
	if need <= 1<<(32-YandexSubnetMinBits) {
		return YandexSubnetMinBits, nil
	}
	return 0, fmt.Errorf("%d hosts in one zone do not fit a /%d subnet", hosts, YandexSubnetMinBits)
}
