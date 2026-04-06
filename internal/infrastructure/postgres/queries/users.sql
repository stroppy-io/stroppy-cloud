-- name: CreateUser :exec
INSERT INTO users (id, username, password_hash, is_root) VALUES ($1, $2, $3, $4);

-- name: GetUser :one
SELECT id, username, password_hash, is_root, created_at FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, is_root, created_at FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT id, username, is_root, created_at FROM users ORDER BY created_at;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $1 WHERE id = $2;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
