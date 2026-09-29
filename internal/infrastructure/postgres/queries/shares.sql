-- name: InsertShare :exec
INSERT INTO shares (id, tenant_id, token, target_kind, target_id, target_name, run_ids, scope, title, snapshot, captured_at, expires_at, created_by)
VALUES (@id, @tenant_id, @token, @target_kind, @target_id, @target_name, @run_ids, @scope, @title, @snapshot, @captured_at, @expires_at, @created_by);

-- name: ShareByID :one
SELECT id, tenant_id, token, target_kind, target_id, target_name, run_ids, scope, title, snapshot, captured_at, expires_at, revoked_at, view_count, created_by, created_at, updated_at
FROM shares WHERE id = @id;

-- name: ShareByToken :one
SELECT id, tenant_id, token, target_kind, target_id, target_name, run_ids, scope, title, snapshot, captured_at, expires_at, revoked_at, view_count, created_by, created_at, updated_at
FROM shares WHERE token = @token;

-- name: SharesOfTenant :many
SELECT id, tenant_id, token, target_kind, target_id, target_name, run_ids, scope, title, snapshot, captured_at, expires_at, revoked_at, view_count, created_by, created_at, updated_at
FROM shares
WHERE tenant_id = @tenant_id
  AND (@target_kind::text = '' OR target_kind = @target_kind::text)
  AND (@target_id::text = '' OR target_id::text = @target_id::text)
  AND (NOT @only_active::boolean OR (revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())))
  AND (NOT @only_inactive::boolean OR (revoked_at IS NOT NULL OR (expires_at IS NOT NULL AND expires_at <= now())))
ORDER BY
  CASE WHEN @sort_key::text = 'title' AND NOT @desc::boolean THEN title END ASC,
  CASE WHEN @sort_key::text = 'title' AND @desc::boolean THEN title END DESC,
  CASE WHEN @sort_key::text = 'target' AND NOT @desc::boolean THEN target_name END ASC,
  CASE WHEN @sort_key::text = 'target' AND @desc::boolean THEN target_name END DESC,
  CASE WHEN @sort_key::text = 'scope' AND NOT @desc::boolean THEN scope END ASC,
  CASE WHEN @sort_key::text = 'scope' AND @desc::boolean THEN scope END DESC,
  CASE WHEN @sort_key::text = 'views' AND NOT @desc::boolean THEN view_count END ASC,
  CASE WHEN @sort_key::text = 'views' AND @desc::boolean THEN view_count END DESC,
  CASE WHEN @sort_key::text = 'expires_at' AND NOT @desc::boolean THEN expires_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'expires_at' AND @desc::boolean THEN expires_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  created_at DESC, id
LIMIT @lim OFFSET @off;

-- name: SharesOfTarget :many
SELECT id, title FROM shares WHERE target_kind = @target_kind AND target_id = @target_id AND revoked_at IS NULL ORDER BY created_at DESC;

-- name: UpdateShare :exec
UPDATE shares
SET scope = COALESCE(@scope::text, scope), title = COALESCE(@title::text, title), expires_at = CASE WHEN @set_expires::boolean THEN @expires_at ELSE expires_at END, updated_at = now()
WHERE id = @id;

-- name: SetShareSnapshot :exec
UPDATE shares SET snapshot = @snapshot, captured_at = @captured_at, title = @title, updated_at = now() WHERE id = @id;

-- name: RevokeShare :execrows
UPDATE shares SET revoked_at = now(), updated_at = now() WHERE id = @id AND revoked_at IS NULL;

-- name: BumpShareViews :exec
UPDATE shares SET view_count = view_count + 1 WHERE id = @id;
