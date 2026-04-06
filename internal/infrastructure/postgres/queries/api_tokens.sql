-- name: CreateAPIToken :exec
INSERT INTO tenant_api_tokens (id, tenant_id, name, token_hash, role, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetAPITokenByHash :one
SELECT id, tenant_id, name, role, created_by, expires_at, created_at
FROM tenant_api_tokens
WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListTenantAPITokens :many
SELECT id, tenant_id, name, role, created_by, expires_at, created_at
FROM tenant_api_tokens WHERE tenant_id = $1
ORDER BY created_at;

-- name: DeleteAPIToken :exec
DELETE FROM tenant_api_tokens WHERE id = $1 AND tenant_id = $2;
