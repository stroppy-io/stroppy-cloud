-- name: SaveRun :exec
INSERT INTO runs (id, tenant_id, snapshot, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
ON CONFLICT(id, tenant_id) DO UPDATE SET snapshot = excluded.snapshot, updated_at = NOW();

-- name: LoadRun :one
SELECT id, tenant_id, snapshot, created_at, updated_at FROM runs
WHERE id = $1 AND tenant_id = $2;

-- name: ListRuns :many
SELECT id, tenant_id, snapshot, created_at, updated_at FROM runs
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: DeleteRun :exec
DELETE FROM runs WHERE id = $1 AND tenant_id = $2;
