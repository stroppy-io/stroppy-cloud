package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

// EnsureDefaultUser creates a user with the given username/password if it doesn't exist.
// Does nothing if the user already exists.
func EnsureDefaultUser(ctx context.Context, pool *pgxpool.Pool, username, password, role string) error {
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check default user: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, encrypted_password, role) VALUES ($1, $2, $3, $4)`,
		ulid.Make().String(), username, string(hash), role,
	); err != nil {
		return fmt.Errorf("insert default user: %w", err)
	}

	return nil
}
