-- name: EnsureSettings :one
INSERT INTO tenant_settings (tenant_id) VALUES (@tenant_id)
ON CONFLICT (tenant_id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id
RETURNING tenant_id, run_retention_days, rating_tenant, rating_global, default_keep, notification_emails, updated_at;

-- name: UpdateSettings :one
UPDATE tenant_settings
SET run_retention_days  = COALESCE(@run_retention_days::int, run_retention_days),
    rating_tenant       = COALESCE(@rating_tenant::boolean, rating_tenant),
    rating_global       = COALESCE(@rating_global::boolean, rating_global),
    default_keep        = COALESCE(@default_keep::text, default_keep),
    notification_emails = COALESCE(@notification_emails::jsonb, notification_emails),
    updated_at          = now()
WHERE tenant_id = @tenant_id
RETURNING tenant_id, run_retention_days, rating_tenant, rating_global, default_keep, notification_emails, updated_at;
