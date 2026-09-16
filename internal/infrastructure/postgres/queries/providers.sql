-- name: InsertProviderProfile :exec
INSERT INTO provider_profiles (id, tenant_id, name, kind, settings, secret_names, status, created_by)
VALUES (@id, @tenant_id, @name, @kind, @settings, @secret_names, @status, @created_by);

-- name: ProviderProfileByID :one
SELECT id, tenant_id, name, kind, settings, secret_names, status, status_reason, verified_at, verify_run_id,
       quotas, quotas_observed_at, quotas_unavailable_reason, quotas_scope, created_by, created_at, updated_at
FROM provider_profiles
WHERE id = @id AND deleted_at IS NULL;

-- name: ProviderProfilesOfTenant :many
SELECT id, tenant_id, name, kind, settings, secret_names, status, status_reason, verified_at, verify_run_id,
       quotas, quotas_observed_at, quotas_unavailable_reason, quotas_scope, created_by, created_at, updated_at
FROM provider_profiles
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
ORDER BY created_at;

-- name: ReadyProviderProfiles :many
-- Every ready profile of every live tenant — the periodic quota refresh.
SELECT p.id, p.tenant_id, p.name, p.kind, p.settings, p.secret_names, p.status, p.status_reason, p.verified_at, p.verify_run_id,
       p.quotas, p.quotas_observed_at, p.quotas_unavailable_reason, p.quotas_scope, p.created_by, p.created_at, p.updated_at
FROM provider_profiles p
JOIN tenants t ON t.id = p.tenant_id AND t.deleted_at IS NULL
WHERE p.status = 'ready' AND p.deleted_at IS NULL
ORDER BY p.quotas_observed_at NULLS FIRST;

-- name: UpdateProviderProfile :execrows
UPDATE provider_profiles
SET name       = COALESCE(@name::text, name),
    settings   = COALESCE(@settings::jsonb, settings),
    updated_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetProviderStatus :execrows
UPDATE provider_profiles
SET status = @status, status_reason = @status_reason, verify_run_id = @verify_run_id,
    verified_at = CASE WHEN @status = 'ready' THEN now() ELSE verified_at END,
    updated_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetProviderQuotas :execrows
UPDATE provider_profiles SET quotas = @quotas, quotas_observed_at = @observed_at,
    quotas_unavailable_reason = @unavailable_reason, quotas_scope = @scope, updated_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteProviderProfile :execrows
UPDATE provider_profiles SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;
