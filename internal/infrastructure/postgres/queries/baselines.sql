-- name: SetBaseline :exec
INSERT INTO baselines (name, tenant_id, run_id) VALUES ($1, $2, $3)
ON CONFLICT(name, tenant_id) DO UPDATE SET run_id = excluded.run_id;

-- name: GetBaseline :one
SELECT name, tenant_id, run_id FROM baselines WHERE name = $1 AND tenant_id = $2;

-- name: ListBaselines :many
SELECT name, tenant_id, run_id FROM baselines WHERE tenant_id = $1;

-- name: DeleteBaseline :exec
DELETE FROM baselines WHERE name = $1 AND tenant_id = $2;
