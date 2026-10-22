package provision

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestYandexZonalSubnets(t *testing.T) {
	run := spec.Run{
		Provider: spec.Provider{Settings: json.RawMessage(`{"zone":"ru-central1-a"}`)},
		Network: spec.Network{Subnets: []spec.Subnet{
			{Zone: "ru-central1-d", CIDR: "10.128.0.32/28"},
			{Zone: "ru-central1-a", CIDR: "10.128.0.0/28"},
			{Zone: "ru-central1-b", CIDR: "10.128.0.16/28"},
		}},
		Machines: []spec.Machine{
			{Name: "a", Location: "ru-central1-a"}, {Name: "b", Location: "ru-central1-b"},
			{Name: "d", Location: "ru-central1-d"}, {Name: "runner"},
		},
	}
	subnets, err := yandexSubnets(run, "test-subnet")
	require.NoError(t, err)
	// Zones in spec order, blocks as the server allocated them.
	require.Equal(t, []yandexSubnet{
		{"ru-central1-a", "test-subnet-ru-central1-a", "10.128.0.0/28"},
		{"ru-central1-b", "test-subnet-ru-central1-b", "10.128.0.16/28"},
		{"ru-central1-d", "test-subnet-ru-central1-d", "10.128.0.32/28"},
	}, subnets)

	// A zone without an allocation is refused: no fallback range on a
	// shared network.
	missing := run
	missing.Network.Subnets = run.Network.Subnets[:2]
	_, err = yandexSubnets(missing, "test-subnet")
	require.ErrorContains(t, err, "no subnet allocated for zone ru-central1-b")

	for _, bad := range []string{"::/64", "10.128.0.0/23", "10.128.0.0/29", "10.128.0.1/28"} {
		broken := run
		broken.Network.Subnets = []spec.Subnet{{Zone: "ru-central1-a", CIDR: bad}, run.Network.Subnets[2], run.Network.Subnets[0]}
		_, err = yandexSubnets(broken, "test-subnet")
		require.Error(t, err, bad)
	}
	overlap := run
	overlap.Network.Subnets = []spec.Subnet{{Zone: "ru-central1-a", CIDR: "10.128.0.0/24"}, run.Network.Subnets[0], run.Network.Subnets[2]}
	_, err = yandexSubnets(overlap, "test-subnet")
	require.ErrorContains(t, err, "overlap")

	single := run
	single.Machines = run.Machines[:1]
	subnets, err = yandexSubnets(single, "test-subnet")
	require.NoError(t, err)
	require.Equal(t, []yandexSubnet{{"ru-central1-a", "test-subnet", "10.128.0.0/28"}}, subnets)
}

func TestYandexNetworkReference(t *testing.T) {
	ref, id := yandexNetworkRef(spec.YandexNetwork{Kind: spec.YandexNetworkCreate}, "sc-abc-profile")
	require.Nil(t, id)
	require.Equal(t, "sc-abc-profile-net", ref.Name)
	ref, id = yandexNetworkRef(spec.YandexNetwork{Kind: spec.YandexNetworkExisting, NetworkID: "enp2v5nl4h1mabcdefgh"}, "sc-abc-profile")
	require.Nil(t, ref)
	require.Equal(t, "enp2v5nl4h1mabcdefgh", *id)
}
