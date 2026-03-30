package badger

import (
	"context"
	"testing"

	badgerdb "github.com/dgraph-io/badger/v4"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
)

func openTestDB(t *testing.T) *badgerdb.DB {
	t.Helper()
	opts := badgerdb.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	db, err := badgerdb.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveLoad(t *testing.T) {
	db := openTestDB(t)
	store := NewDAGStorage(db)
	ctx := context.Background()

	snap := &dag.Snapshot{
		GraphJSON: []byte(`{"nodes":[{"id":"a","type":"x"}]}`),
		Nodes: []dag.NodeStatus{
			{ID: "a", Status: dag.StatusDone},
		},
	}

	if err := store.Save(ctx, "run-1", snap); err != nil {
		t.Fatal(err)
	}

	got, err := store.Load(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if len(got.Nodes) != 1 || got.Nodes[0].Status != dag.StatusDone {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

func TestLoadNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewDAGStorage(db)

	got, err := store.Load(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestOverwrite(t *testing.T) {
	db := openTestDB(t)
	store := NewDAGStorage(db)
	ctx := context.Background()

	snap1 := &dag.Snapshot{
		GraphJSON: []byte(`{}`),
		Nodes:     []dag.NodeStatus{{ID: "a", Status: dag.StatusPending}},
	}
	snap2 := &dag.Snapshot{
		GraphJSON: []byte(`{}`),
		Nodes:     []dag.NodeStatus{{ID: "a", Status: dag.StatusDone}},
	}

	if err := store.Save(ctx, "run-1", snap1); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, "run-1", snap2); err != nil {
		t.Fatal(err)
	}

	got, err := store.Load(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Nodes[0].Status != dag.StatusDone {
		t.Fatalf("expected overwritten status Done, got %s", got.Nodes[0].Status)
	}
}
