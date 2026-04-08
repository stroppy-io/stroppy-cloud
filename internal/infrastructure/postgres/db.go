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
	// Ensure root user + default tenant exist.
	if err := seedUsersAndTenants(ctx, pool); err != nil {
		return err
	}

	// Seed built-in packages and presets for every tenant that is missing them.
	return seedBuiltins(ctx, pool)
}

func seedUsersAndTenants(ctx context.Context, pool *pgxpool.Pool) error {
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

// seedBuiltins ensures every tenant has built-in packages and presets.
// Runs on every startup — idempotent via ON CONFLICT DO NOTHING and count checks.
func seedBuiltins(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, "SELECT id FROM tenants")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		tenantIDs = append(tenantIDs, id)
	}

	for _, tenantID := range tenantIDs {
		// Seed packages.
		var pkgCount int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM packages WHERE tenant_id = $1", tenantID).Scan(&pkgCount)
		if pkgCount == 0 {
			for _, bp := range types.BuiltinPackages() {
				apt := bp.AptPackages
				if apt == nil {
					apt = []string{}
				}
				pre := bp.PreInstall
				if pre == nil {
					pre = []string{}
				}
				_, _ = pool.Exec(ctx,
					`INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages, pre_install, custom_repo, custom_repo_key, deb_filename)
					 VALUES ($1, $2, $3, $4, $5, $6, TRUE, $7, $8, $9, $10, '')
					 ON CONFLICT DO NOTHING`,
					uuid.New().String(), tenantID, bp.Name, bp.Description,
					bp.DbKind, bp.DbVersion, apt, pre, bp.CustomRepo, bp.CustomRepoKey,
				)
			}
		}

		// Seed presets.
		var presetCount int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM presets WHERE tenant_id = $1", tenantID).Scan(&presetCount)
		if presetCount == 0 {
			for _, bp := range types.BuiltinPresets() {
				topoJSON, err := bp.TopologyJSON()
				if err != nil {
					continue
				}
				_, _ = pool.Exec(ctx,
					`INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin)
					 VALUES ($1, $2, $3, $4, $5, $6, TRUE)
					 ON CONFLICT DO NOTHING`,
					uuid.New().String(), tenantID, bp.Name, bp.Description,
					bp.DbKind, topoJSON,
				)
			}
		}
	}

	return nil
}
