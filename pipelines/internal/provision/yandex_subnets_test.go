package provision

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestYandexZonalSubnets(t *testing.T) {
	run := spec.Run{Network: spec.Network{CIDR: "10.130.0.0/24"}, Machines: []spec.Machine{
		{Name: "a", Location: "ru-central1-a"}, {Name: "b", Location: "ru-central1-b"},
		{Name: "d", Location: "ru-central1-d"}, {Name: "runner"},
	}}
	subnets, err := yandexSubnets(run, "ru-central1-a", "test-subnet")
	require.NoError(t, err)
	require.Len(t, subnets, 3)
	require.Equal(t, []yandexSubnet{
		{"ru-central1-a", "test-subnet-ru-central1-a", "10.130.0.0/26"},
		{"ru-central1-b", "test-subnet-ru-central1-b", "10.130.0.64/26"},
		{"ru-central1-d", "test-subnet-ru-central1-d", "10.130.0.128/26"},
	}, subnets)
	for i, sub := range subnets {
		for _, other := range subnets[i+1:] {
			require.False(t, netip.MustParsePrefix(sub.CIDR).Overlaps(netip.MustParsePrefix(other.CIDR)))
		}
	}
	run.Network.CIDR = "10.130.0.0/28"
	_, err = yandexSubnets(run, "ru-central1-a", "test-subnet")
	require.ErrorContains(t, err, "too small")
	run.Network.CIDR = "::/64"
	_, err = yandexSubnets(run, "ru-central1-a", "test-subnet")
	require.ErrorContains(t, err, "IPv4")
	run.Network.CIDR = "10.130.0.0/24"
	run.Machines = run.Machines[:1]
	subnets, err = yandexSubnets(run, "ru-central1-a", "test-subnet")
	require.NoError(t, err)
	require.Equal(t, []yandexSubnet{{"ru-central1-a", "test-subnet", "10.130.0.0/24"}}, subnets)
}
