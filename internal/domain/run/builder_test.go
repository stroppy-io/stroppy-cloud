package run

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func baseDeps() Deps {
	return Deps{
		State: NewState(),
	}
}

func postgresSingleCfg() types.RunConfig {
	topo := types.PostgresPresets[types.PostgresSingle]
	return types.RunConfig{
		ID:       "test-pg-single",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
		Database: types.DatabaseConfig{
			Kind:     types.DatabasePostgres,
			Version:  "16",
			Postgres: &topo,
		},
		Stroppy: types.StroppyConfig{Workload: "ycsb", Duration: "30s", Workers: 4},
	}
}

func TestBuild_PostgresSingle(t *testing.T) {
	cfg := postgresSingleCfg()
	graph, reg, err := Build(cfg, baseDeps())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}

	// Single PG has these core phases: network, machines, install_db, configure_db,
	// install_monitor, configure_monitor, install_stroppy, run_stroppy, teardown.
	nodeIDs := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeIDs[n.ID] = true
	}

	required := []string{
		"network", "machines", "install_db", "configure_db",
		"install_monitor", "configure_monitor",
		"install_stroppy", "run_stroppy", "teardown",
	}
	for _, id := range required {
		if !nodeIDs[id] {
			t.Errorf("missing expected node %q", id)
		}
	}
}

func TestBuild_PostgresHA(t *testing.T) {
	topo := types.PostgresPresets[types.PostgresHA]
	cfg := postgresSingleCfg()
	cfg.ID = "test-pg-ha"
	cfg.Database.Postgres = &topo

	graph, _, err := Build(cfg, baseDeps())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(graph.Nodes) < 9 {
		t.Errorf("expected at least 9 nodes for HA, got %d", len(graph.Nodes))
	}

	// HA with HAProxy should include proxy phases.
	nodeIDs := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeIDs[n.ID] = true
	}
	if !nodeIDs["install_proxy"] {
		t.Error("expected install_proxy node for HA topology with HAProxy")
	}
	if !nodeIDs["configure_proxy"] {
		t.Error("expected configure_proxy node for HA topology with HAProxy")
	}
}

func TestBuild_PostgresScale(t *testing.T) {
	topo := types.PostgresPresets[types.PostgresScale]
	cfg := postgresSingleCfg()
	cfg.ID = "test-pg-scale"
	cfg.Database.Postgres = &topo

	graph, _, err := Build(cfg, baseDeps())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Scale has pgbouncer + haproxy so more nodes.
	nodeIDs := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeIDs[n.ID] = true
	}
	if !nodeIDs["install_pgbouncer"] {
		t.Error("expected install_pgbouncer node for scale topology")
	}
	if !nodeIDs["install_proxy"] {
		t.Error("expected install_proxy node for scale topology")
	}
}

func TestBuild_UnsupportedKindErrors(t *testing.T) {
	cfg := postgresSingleCfg()
	cfg.Database.Kind = "cockroach"
	cfg.Database.Postgres = nil

	_, _, err := Build(cfg, baseDeps())
	if err == nil {
		t.Fatal("expected error for unsupported database kind, got nil")
	}
}

func TestBuild_MySQLSingle(t *testing.T) {
	topo := types.MySQLPresets[types.MySQLSingle]
	cfg := types.RunConfig{
		ID:       "test-mysql-single",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
		Database: types.DatabaseConfig{
			Kind:    types.DatabaseMySQL,
			Version: "8.0",
			MySQL:   &topo,
		},
		Stroppy: types.StroppyConfig{Workload: "ycsb", Duration: "30s", Workers: 4},
	}

	graph, _, err := Build(cfg, baseDeps())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(graph.Nodes) < 9 {
		t.Errorf("expected at least 9 nodes, got %d", len(graph.Nodes))
	}
}

func TestBuild_PicodataSingle(t *testing.T) {
	topo := types.PicodataPresets[types.PicodataSingle]
	cfg := types.RunConfig{
		ID:       "test-pico-single",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
		Database: types.DatabaseConfig{
			Kind:     types.DatabasePicodata,
			Version:  "25.3",
			Picodata: &topo,
		},
		Stroppy: types.StroppyConfig{Workload: "ycsb", Duration: "30s", Workers: 4},
	}

	graph, _, err := Build(cfg, baseDeps())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(graph.Nodes) < 9 {
		t.Errorf("expected at least 9 nodes, got %d", len(graph.Nodes))
	}
}
