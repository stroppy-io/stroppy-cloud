package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYandexNetworkOf(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		want           YandexNetwork
		pool           string
		sharedEgress   bool
	}{
		{"legacy create", `{"network":{"kind":"create","subnet_cidr":"10.130.0.0/24"}}`, YandexNetwork{Kind: "create", PublicIPs: true}, DefaultYandexPool, false},
		{"create private", `{"network":{"kind":"create","pool_cidr":"10.64.0.0/20"},"public_ips":false}`, YandexNetwork{Kind: "create", PoolCIDR: "10.64.0.0/20"}, "10.64.0.0/20", true},
		{"existing without routes", `{"network":{"kind":"existing","network_id":"n","pool_cidr":"10.1.0.0/24"},"public_ips":false}`, YandexNetwork{Kind: "existing", NetworkID: "n", PoolCIDR: "10.1.0.0/24"}, "10.1.0.0/24", false},
		{"existing with routes", `{"network":{"kind":"existing","network_id":"n","pool_cidr":"10.1.0.0/24","route_table_id":"r"}}`, YandexNetwork{Kind: "existing", NetworkID: "n", PoolCIDR: "10.1.0.0/24", RouteTableID: "r", PublicIPs: true}, "10.1.0.0/24", true},
		{"no network block", `{}`, YandexNetwork{Kind: "create", PublicIPs: true}, DefaultYandexPool, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := YandexNetworkOf(json.RawMessage(tc.settings))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.pool, got.Pool())
			require.Equal(t, tc.sharedEgress, got.SharedEgress())
		})
	}
	_, err := YandexNetworkOf(json.RawMessage(`{"network":{"kind":"existing"}}`))
	require.Error(t, err)
	_, err = YandexNetworkOf(json.RawMessage(`{"network":{"kind":"vpn"}}`))
	require.Error(t, err)
}

func TestProfileNetworkNamesFitYandexNames(t *testing.T) {
	pc := ProviderConfigName("t-acme", "b413b185-61ca-4d81-abf2-0b760aa59ed5")
	for _, name := range []string{ProfileNetworkName(pc), ProfileGatewayName(pc), ProfileRouteTableName(pc)} {
		require.LessOrEqual(t, len(name), 63, name)
	}
}

func TestYandexZones(t *testing.T) {
	run := Run{
		Provider: Provider{Settings: json.RawMessage(`{"zone":"ru-central1-d"}`)},
		Machines: []Machine{{Name: "db-1", Location: "ru-central1-a"}, {Name: "db-2", Location: "ru-central1-b"}, {Name: "db-3", Location: "ru-central1-a"}, {Name: "runner"}},
	}
	zones, err := YandexZones(run)
	require.NoError(t, err)
	require.Equal(t, []ZoneDemand{{"ru-central1-a", 2}, {"ru-central1-b", 1}, {"ru-central1-d", 1}}, zones)

	run.ManagedYDB = &ManagedYDB{Type: "dedicated", NodeCount: 3, Zones: []string{"ru-central1-d", "ru-central1-e"}}
	zones, err = YandexZones(run)
	require.NoError(t, err)
	require.Equal(t, []ZoneDemand{{"ru-central1-a", 2}, {"ru-central1-b", 1}, {"ru-central1-d", 3}, {"ru-central1-e", 2}}, zones)

	zones, err = YandexZones(Run{Provider: run.Provider})
	require.NoError(t, err)
	require.Equal(t, []ZoneDemand{{"ru-central1-d", 0}}, zones)

	_, err = YandexZones(Run{Machines: []Machine{{Name: "lost"}}})
	require.Error(t, err)
}

func TestYandexSubnetBits(t *testing.T) {
	for hosts, bits := range map[int]int{0: 28, 1: 28, 4: 28, 5: 27, 12: 27, 13: 26, 60: 25, 124: 24, 125: 24, 252: 24} {
		got, err := YandexSubnetBits(hosts)
		require.NoError(t, err, hosts)
		require.Equal(t, bits, got, hosts)
	}
	_, err := YandexSubnetBits(253)
	require.Error(t, err)
}
