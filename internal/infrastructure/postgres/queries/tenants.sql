-- name: CreateTenant :exec
INSERT INTO tenants (id, name) VALUES ($1, $2);

-- name: GetTenant :one
SELECT id, name, account_id, created_at FROM tenants WHERE id = $1;

-- name: GetTenantByName :one
SELECT id, name, account_id, created_at FROM tenants WHERE name = $1;

-- name: ListTenants :many
SELECT id, name, account_id, created_at FROM tenants ORDER BY created_at;

-- name: DeleteTenant :exec
DELETE FROM tenants WHERE id = $1;
