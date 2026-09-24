-- name: InsertProviderProfile :exec
INSERT INTO provider_profiles (id, tenant_id, name, kind, settings, secret_names, status, created_by, verify_run_id)
VALUES (@id, @tenant_id, @name, @kind, @settings, @secret_names, @status, @created_by, @verify_run_id);

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

-- name: BeginProviderOperation :execrows
UPDATE provider_profiles p
SET status = @status, status_reason = '', verify_run_id = @run_id, updated_at = now()
WHERE p.id = @id AND p.deleted_at IS NULL AND p.status IN ('ready', 'failed', 'delete_failed')
 AND (@status::text = 'deleting' OR p.status <> 'delete_failed')
 AND NOT EXISTS (
  SELECT 1 FROM runs r WHERE r.tenant_id = p.tenant_id
   AND r.snapshot->'provider_profile'->>'id' = p.id::text
   AND (r.status IN ('pending','running','cancelling') OR r.stand_kept)
 );

-- name: FinishProviderOperation :execrows
UPDATE provider_profiles SET status = @status, status_reason = @reason,
 verified_at = CASE WHEN @status::text = 'ready' THEN now() ELSE verified_at END, updated_at=now()
WHERE id=@id AND verify_run_id=@run_id AND deleted_at IS NULL;

-- name: PendingProviderOperations :many
SELECT id, tenant_id, name, kind, settings, secret_names, status, status_reason, verified_at, verify_run_id,
 quotas, quotas_observed_at, quotas_unavailable_reason, quotas_scope, created_by, created_at, updated_at
FROM provider_profiles WHERE deleted_at IS NULL AND status='verifying' AND verify_run_id<>'';

-- name: LockProviderLifecycle :one
SELECT status FROM provider_profiles WHERE id=@id AND deleted_at IS NULL FOR UPDATE;

-- name: LockTenantLifecycle :one
SELECT retiring FROM tenants WHERE id=@id AND deleted_at IS NULL FOR UPDATE;

-- name: BeginTenantRetirement :execrows
UPDATE tenants SET retiring=true, updated_at=now()
WHERE id=@id AND deleted_at IS NULL
 AND NOT EXISTS (SELECT 1 FROM runs WHERE tenant_id=@id AND (status IN ('pending','running','cancelling') OR stand_kept))
 AND NOT EXISTS (SELECT 1 FROM provider_profiles WHERE tenant_id=@id AND deleted_at IS NULL AND status='verifying');

-- name: SetProviderSecretNames :exec
UPDATE provider_profiles SET secret_names=@secret_names WHERE id=@id AND deleted_at IS NULL;
