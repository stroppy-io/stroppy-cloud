-- name: GetTenantSettings :one
SELECT tenant_id, settings FROM tenant_settings WHERE tenant_id = $1;

-- name: UpsertTenantSettings :exec
INSERT INTO tenant_settings (tenant_id, settings) VALUES ($1, $2)
ON CONFLICT(tenant_id) DO UPDATE SET settings = excluded.settings;
