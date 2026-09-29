-- name: ProfileByID :one
SELECT id, email, display_name, avatar, is_platform_admin, preferences, notifications, created_at, updated_at
FROM profiles
WHERE id = @id;

-- name: EnsureProfile :one
-- First authenticated call creates the row; later calls only refresh the
-- email when IAM reports a new one (empty = keep). xmax = 0 marks an insert.
INSERT INTO profiles (id, email)
VALUES (@id, @email)
ON CONFLICT (id) DO UPDATE
    SET email = CASE WHEN EXCLUDED.email = '' THEN profiles.email ELSE EXCLUDED.email END,
        updated_at = now()
RETURNING id, email, display_name, avatar, is_platform_admin, preferences, notifications, created_at, updated_at,
          (xmax = 0) AS created;

-- name: UpdateProfile :one
-- Partial update: a NULL argument keeps the column.
UPDATE profiles
SET display_name  = COALESCE(@display_name::text, display_name),
    avatar        = COALESCE(@avatar::text, avatar),
    preferences   = COALESCE(@preferences::jsonb, preferences),
    notifications = COALESCE(@notifications::jsonb, notifications),
    updated_at    = now()
WHERE id = @id
RETURNING id, email, display_name, avatar, is_platform_admin, preferences, notifications, created_at, updated_at;

-- name: SetProfileEmail :execrows
UPDATE profiles SET email = @email, updated_at = now() WHERE id = @id;

-- name: DeleteProfile :execrows
DELETE FROM profiles WHERE id = @id;

-- name: ProfileRefs :many
-- Display data for a batch of subjects (UserRef enrichment); missing ids
-- are simply absent. The email is disclosed only to the subject itself, to
-- a co-member of at least one tenant (confined to scope_tenant when set —
-- API tokens) or when reveal is set (platform admin); otherwise ''.
SELECT p.id,
       CASE WHEN @reveal::boolean
                 OR p.id = @viewer::uuid
                 OR EXISTS (SELECT 1
                            FROM tenant_members v
                            JOIN tenant_members m ON m.tenant_id = v.tenant_id
                            WHERE m.user_id = p.id
                              AND v.user_id = @viewer::uuid
                              AND (@scope_tenant::uuid = '00000000-0000-0000-0000-000000000000'::uuid
                                   OR v.tenant_id = @scope_tenant::uuid))
            THEN p.email ELSE '' END AS email,
       p.display_name,
       p.avatar
FROM profiles p
WHERE p.id = ANY(@ids::uuid[]);
