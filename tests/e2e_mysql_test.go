//go:build integration

package tests

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

func mysqlCfg(id string, preset types.MySQLPreset, version string) types.RunConfig {
	topo := types.MySQLPresets[preset]
	dbCount := 1
	if len(topo.Replicas) > 0 {
		dbCount += topo.Replicas[0].Count
	}
	machines := []types.MachineSpec{
		{Role: types.RoleDatabase, Count: dbCount, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
		{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
		{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
	}
	if topo.ProxySQL != nil {
		machines = append(machines, types.MachineSpec{Role: types.RoleProxy, Count: topo.ProxySQL.Count, CPUs: 2, MemoryMB: 2048, DiskGB: 20})
	}
	return types.RunConfig{
		ID:       id,
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: machines,
		Database: types.DatabaseConfig{Kind: types.DatabaseMySQL, Version: version, MySQL: &topo},
		Monitor:  types.MonitorConfig{},
		// stroppy has no MySQL driver — run_stroppy will fail, that's expected
		Stroppy: types.StroppyConfig{Version: "3.1.0", Workload: "simple", Duration: "5s", Workers: 1},
	}
}

// MySQL E2E tests: install + configure succeed, run_stroppy expected to fail (no driver).
// We test that the infrastructure phases complete correctly.

func TestE2E_MySQL_Single(t *testing.T) {
	ts := startE2E(t)
	cfg := mysqlCfg("e2e-my-s", types.MySQLSingle, "8.0")
	// Expect failure at run_stroppy (no MySQL driver in stroppy)
	ts.run(t, cfg, true)
}

func TestE2E_MySQL_Replica(t *testing.T) {
	ts := startE2E(t)
	// Test replication without ProxySQL (ProxySQL repo often times out).
	topo := types.MySQLPresets[types.MySQLReplica]
	topo.ProxySQL = nil // disable ProxySQL for Docker testing
	cfg := types.RunConfig{
		ID:       "e2e-my-r",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: []types.MachineSpec{
			{Role: types.RoleDatabase, Count: 3, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
			{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
			{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
		},
		Database: types.DatabaseConfig{Kind: types.DatabaseMySQL, Version: "8.0", MySQL: &topo},
		Monitor:  types.MonitorConfig{},
		Stroppy:  types.StroppyConfig{Version: "3.1.0", Workload: "simple", Duration: "5s", Workers: 1},
	}
	ts.run(t, cfg, true)
}

func TestE2E_MySQL_Group(t *testing.T) {
	ts := startE2E(t)
	// Test group replication without ProxySQL.
	topo := types.MySQLPresets[types.MySQLGroup]
	topo.ProxySQL = nil
	cfg := types.RunConfig{
		ID:       "e2e-my-g",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: []types.MachineSpec{
			{Role: types.RoleDatabase, Count: 3, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
			{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
			{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
		},
		Database: types.DatabaseConfig{Kind: types.DatabaseMySQL, Version: "8.0", MySQL: &topo},
		Monitor:  types.MonitorConfig{},
		Stroppy:  types.StroppyConfig{Version: "3.1.0", Workload: "simple", Duration: "5s", Workers: 1},
	}
	ts.run(t, cfg, true)
}
