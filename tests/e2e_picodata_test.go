//go:build integration

package tests

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

func picodataCfg(id string, preset types.PicodataPreset, version string) types.RunConfig {
	topo := types.PicodataPresets[preset]
	dbCount := 1
	if len(topo.Instances) > 0 {
		dbCount = topo.Instances[0].Count
	}
	machines := []types.MachineSpec{
		{Role: types.RoleDatabase, Count: dbCount, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
		{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
		{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
	}
	if topo.HAProxy != nil {
		machines = append(machines, types.MachineSpec{Role: types.RoleProxy, Count: topo.HAProxy.Count, CPUs: 2, MemoryMB: 2048, DiskGB: 20})
	}
	return types.RunConfig{
		ID:       id,
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: machines,
		Database: types.DatabaseConfig{Kind: types.DatabasePicodata, Version: version, Picodata: &topo},
		Monitor:  types.MonitorConfig{},
		// Picodata speaks pgproto — stroppy postgres driver should work via port 4327
		Stroppy: types.StroppyConfig{Version: "3.1.0", Workload: "simple", Duration: "5s", Workers: 1},
	}
}

// Picodata E2E: picodata speaks pgproto so stroppy postgres driver can connect.
// If picodata packages are unavailable, tests will fail at install phase — that's expected
// in environments without picodata repo access.

func TestE2E_Picodata_Single(t *testing.T) {
	ts := startE2E(t)
	cfg := picodataCfg("e2e-pi-s", types.PicodataSingle, "25.3")
	// May fail if picodata repo is not accessible; expected in CI without picodata access
	ts.run(t, cfg, true)
}

func TestE2E_Picodata_Cluster(t *testing.T) {
	ts := startE2E(t)
	cfg := picodataCfg("e2e-pi-c", types.PicodataCluster, "25.3")
	ts.run(t, cfg, true)
}
