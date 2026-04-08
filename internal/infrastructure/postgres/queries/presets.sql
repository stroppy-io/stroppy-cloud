-- name: CreatePreset :exec
INSERT INTO presets (id, tenant_id, name, description, db_kind, topology, is_builtin, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW());

-- name: GetPreset :one
SELECT id, tenant_id, name, description, db_kind, topology, is_builtin, created_at, updated_at
FROM presets WHERE id = $1 AND tenant_id = $2;

-- name: ListPresetsTenant :many
SELECT id, tenant_id, name, description, db_kind, topology, is_builtin, created_at, updated_at
FROM presets WHERE tenant_id = $1 ORDER BY is_builtin DESC, name;

-- name: ListPresetsByKind :many
SELECT id, tenant_id, name, description, db_kind, topology, is_builtin, created_at, updated_at
FROM presets WHERE tenant_id = $1 AND db_kind = $2 ORDER BY is_builtin DESC, name;

-- name: UpdatePreset :exec
UPDATE presets SET name = $3, description = $4, topology = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND is_builtin = FALSE;

-- name: DeletePreset :exec
DELETE FROM presets WHERE id = $1 AND tenant_id = $2 AND is_builtin = FALSE;
