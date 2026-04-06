package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open connects to PostgreSQL, runs migrations, and seeds the root user.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	if err := runMigrations(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}

	if err := seed(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres seed: %w", err)
	}

	return pool, nil
}

func runMigrations(dsn string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+stripScheme(dsn))
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// stripScheme removes "postgres://" or "postgresql://" prefix since golang-migrate pgx5 driver
// expects its own scheme.
func stripScheme(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if len(dsn) > len(prefix) && dsn[:len(prefix)] == prefix {
			return dsn[len(prefix):]
		}
	}
	return dsn
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	// Skip if users already exist AND at least one tenant exists.
	var userCount, tenantCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		return err
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&tenantCount); err != nil {
		return err
	}
	if userCount > 0 && tenantCount > 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Ensure root user exists.
	var userID string
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE username = 'admin'").Scan(&userID)
	if err != nil {
		// User doesn't exist — create.
		adminPass := os.Getenv("STROPPY_ADMIN_PASSWORD")
		if adminPass == "" {
			adminPass = "admin"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		userID = uuid.New().String()
		if _, err := tx.Exec(ctx,
			"INSERT INTO users (id, username, password_hash, is_root) VALUES ($1, $2, $3, $4)",
			userID, "admin", string(hash), true,
		); err != nil {
			return err
		}
	}

	// Ensure default tenant exists.
	var tenantID string
	err = tx.QueryRow(ctx, "SELECT id FROM tenants WHERE name = 'default'").Scan(&tenantID)
	if err != nil {
		tenantID = uuid.New().String()
		if _, err := tx.Exec(ctx,
			"INSERT INTO tenants (id, name) VALUES ($1, $2)",
			tenantID, "default",
		); err != nil {
			return err
		}
		// Create default settings with sensible defaults (grafana embed, monitoring versions, etc.)
		defaultSettings := types.DefaultServerSettings()
		settingsJSON, _ := json.Marshal(defaultSettings)
		if _, err := tx.Exec(ctx,
			"INSERT INTO tenant_settings (tenant_id, settings) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			tenantID, string(settingsJSON),
		); err != nil {
			return err
		}
	}

	// Ensure admin is owner of default tenant.
	if _, err := tx.Exec(ctx,
		"INSERT INTO tenant_members (tenant_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		tenantID, userID, "owner",
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
