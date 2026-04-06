package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}
	ctx := context.Background()
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestOpen(t *testing.T) {
	pool := testPool(t)

	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM tenants").Scan(&count)
	if err != nil {
		t.Fatalf("query tenants: %v", err)
	}
}

func TestSeedCreatesRootUser(t *testing.T) {
	pool := testPool(t)

	var username string
	var isRoot bool
	err := pool.QueryRow(context.Background(), "SELECT username, is_root FROM users WHERE is_root = true LIMIT 1").Scan(&username, &isRoot)
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	if username != "admin" || !isRoot {
		t.Fatalf("expected root admin, got username=%q is_root=%v", username, isRoot)
	}
}
