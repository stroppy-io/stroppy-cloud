package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

func testDB(t *testing.T) *RunStorage {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}
	ctx := context.Background()
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })

	_, err = pool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ('t1', 'test-tenant') ON CONFLICT DO NOTHING")
	if err != nil {
		t.Fatal(err)
	}

	return NewRunStorage(pool)
}

// Ensure the pool variable is used (satisfies compile check).
var _ *pgxpool.Pool

func TestRunStorage_SaveAndLoad(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	snap := &dag.Snapshot{
		Nodes: []dag.NodeStatus{{ID: "n1", Status: dag.StatusDone}},
	}

	if err := s.Save(ctx, "t1", "run-1", snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load(ctx, "t1", "run-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Nodes) != 1 || loaded.Nodes[0].ID != "n1" {
		t.Fatalf("unexpected nodes: %+v", loaded.Nodes)
	}

	// Cleanup
	_ = s.Delete(ctx, "t1", "run-1")
}

func TestRunStorage_LoadNotFound(t *testing.T) {
	s := testDB(t)
	snap, err := s.Load(context.Background(), "t1", "nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap != nil {
		t.Fatal("expected nil for missing run")
	}
}

func TestRunStorage_ListByTenant(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	snap := &dag.Snapshot{Nodes: []dag.NodeStatus{{ID: "n1", Status: dag.StatusDone}}}

	s.Save(ctx, "t1", "run-list-1", snap)
	s.Save(ctx, "t1", "run-list-2", snap)

	runs, err := s.List(ctx, "t1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) < 2 {
		t.Fatalf("expected at least 2 runs, got %d", len(runs))
	}

	// Cleanup
	_ = s.Delete(ctx, "t1", "run-list-1")
	_ = s.Delete(ctx, "t1", "run-list-2")
}

func TestRunStorage_DeleteRun(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	snap := &dag.Snapshot{Nodes: []dag.NodeStatus{{ID: "n1", Status: dag.StatusDone}}}

	s.Save(ctx, "t1", "run-del-1", snap)
	if err := s.Delete(ctx, "t1", "run-del-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	loaded, _ := s.Load(ctx, "t1", "run-del-1")
	if loaded != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestRunStorage_Baselines(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	if err := s.SetBaseline(ctx, "t1", "pg16", "run-1"); err != nil {
		t.Fatalf("SetBaseline: %v", err)
	}

	runID, err := s.GetBaseline(ctx, "t1", "pg16")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if runID != "run-1" {
		t.Fatalf("expected run-1, got %s", runID)
	}

	baselines, err := s.ListBaselines(ctx, "t1")
	if err != nil {
		t.Fatalf("ListBaselines: %v", err)
	}
	if len(baselines) < 1 {
		t.Fatalf("expected at least 1 baseline, got %d", len(baselines))
	}

	// Cleanup
	_, _ = s.pool.Exec(ctx, "DELETE FROM baselines WHERE tenant_id = 't1' AND name = 'pg16'")
}
