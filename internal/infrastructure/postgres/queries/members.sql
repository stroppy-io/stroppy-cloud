-- name: AddMember :exec
INSERT INTO tenant_members (tenant_id, user_id, role) VALUES ($1, $2, $3);

-- name: UpdateMemberRole :exec
UPDATE tenant_members SET role = $1 WHERE tenant_id = $2 AND user_id = $3;

-- name: RemoveMember :exec
DELETE FROM tenant_members WHERE tenant_id = $1 AND user_id = $2;

-- name: GetMember :one
SELECT tenant_id, user_id, role, created_at FROM tenant_members
WHERE tenant_id = $1 AND user_id = $2;

-- name: ListTenantMembers :many
SELECT tm.tenant_id, tm.user_id, tm.role, tm.created_at, u.username
FROM tenant_members tm JOIN users u ON tm.user_id = u.id
WHERE tm.tenant_id = $1
ORDER BY tm.created_at;

-- name: ListUserTenants :many
SELECT tm.tenant_id, tm.role, t.name AS tenant_name
FROM tenant_members tm JOIN tenants t ON tm.tenant_id = t.id
WHERE tm.user_id = $1
ORDER BY t.name;
