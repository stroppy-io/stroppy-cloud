-- name: CreateSharedRun :exec
INSERT INTO shared_runs (token, run_id, tenant_id, snapshot, metrics, created_by)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSharedRun :one
SELECT token, run_id, snapshot, metrics, created_at FROM shared_runs
WHERE token = $1;

-- name: DeleteSharedRun :exec
DELETE FROM shared_runs WHERE token = $1 AND tenant_id = $2;

-- name: ListSharedRuns :many
SELECT token, run_id, created_at FROM shared_runs
WHERE tenant_id = $1 ORDER BY created_at DESC;
