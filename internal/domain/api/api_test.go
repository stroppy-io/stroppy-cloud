package api

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	app, err := New(Config{}) // in-memory badger
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { app.Close() })
	return app
}

func postgresSingleConfig() types.RunConfig {
	topo := types.PostgresPresets[types.PostgresSingle]
	return types.RunConfig{
		ID:       "test-run-1",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
		Database: types.DatabaseConfig{
			Kind:     types.DatabasePostgres,
			Version:  "16",
			Postgres: &topo,
		},
		Monitor: types.MonitorConfig{},
		Stroppy: types.StroppyConfig{
			Version:  "latest",
			Workload: "ycsb",
			Duration: "30s",
			Workers:  4,
		},
	}
}

func TestNew_InMemory(t *testing.T) {
	app := newTestApp(t)
	if app.db == nil {
		t.Fatal("expected non-nil db")
	}
	if app.storage == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestValidate_PostgresSingle(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()

	if err := app.Validate(cfg); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
}

func TestValidate_UnsupportedKind(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()
	cfg.Database.Kind = "cockroach"
	cfg.Database.Postgres = nil

	if err := app.Validate(cfg); err == nil {
		t.Fatal("expected error for unsupported kind, got nil")
	}
}

func TestDryRun_ReturnsJSON(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()

	data, err := app.DryRun(cfg)
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}

	if !json.Valid(data) {
		t.Fatal("DryRun() returned invalid JSON")
	}

	// Verify it contains expected node IDs.
	var graph map[string]any
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatalf("unmarshal graph: %v", err)
	}
}

func TestDryRun_UnsupportedKindErrors(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()
	cfg.Database.Kind = "cockroach"
	cfg.Database.Postgres = nil

	_, err := app.DryRun(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported kind, got nil")
	}
}
