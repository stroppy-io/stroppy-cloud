-- name: AdminTenants :many
SELECT t.id, t.slug, t.name, t.description, t.public_name, t.status, t.owner_id, t.graphene_namespace, t.created_at, t.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id)::int AS member_count
FROM tenants t
WHERE t.deleted_at IS NULL
  AND (@search::text = '' OR t.name ILIKE '%' || @search::text || '%' OR t.slug ILIKE '%' || @search::text || '%')
  AND (@status::text = '' OR t.status = @status::text)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN t.name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN t.name END DESC,
  CASE WHEN @sort_key::text = 'slug' AND NOT @desc::boolean THEN t.slug END ASC,
  CASE WHEN @sort_key::text = 'slug' AND @desc::boolean THEN t.slug END DESC,
  CASE WHEN @sort_key::text = 'status' AND NOT @desc::boolean THEN t.status END ASC,
  CASE WHEN @sort_key::text = 'status' AND @desc::boolean THEN t.status END DESC,
  CASE WHEN @sort_key::text = 'members' AND NOT @desc::boolean THEN (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id) END ASC,
  CASE WHEN @sort_key::text = 'members' AND @desc::boolean THEN (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id) END DESC,
  CASE WHEN @sort_key::text = 'runs' AND NOT @desc::boolean THEN (SELECT count(*) FROM runs r WHERE r.tenant_id = t.id AND r.deleted_at IS NULL) END ASC,
  CASE WHEN @sort_key::text = 'runs' AND @desc::boolean THEN (SELECT count(*) FROM runs r WHERE r.tenant_id = t.id AND r.deleted_at IS NULL) END DESC,
  CASE WHEN @sort_key::text = 'last_activity' AND NOT @desc::boolean THEN (SELECT max(a.at) FROM audit_log a WHERE a.tenant_id = t.id) END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_activity' AND @desc::boolean THEN (SELECT max(a.at) FROM audit_log a WHERE a.tenant_id = t.id) END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN t.created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN t.created_at END DESC,
  t.created_at DESC, t.id
LIMIT @lim OFFSET @off;

-- name: TenantStatusCounts :one
SELECT count(*) FILTER (WHERE status = 'active')::int AS active,
       count(*) FILTER (WHERE status = 'suspended')::int AS suspended,
       count(*) FILTER (WHERE status = 'orphaned')::int AS orphaned
FROM tenants WHERE deleted_at IS NULL;

-- name: TenantSuspendedReason :one
SELECT suspended_reason FROM tenants WHERE id = @id;

-- name: SetTenantSuspended :execrows
UPDATE tenants SET status = @status, suspended_reason = @reason, updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: TenantCounters :one
SELECT (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = @tenant_id)::int AS members,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL)::int AS runs_total,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL AND r.status IN ('pending', 'running', 'cancelling'))::int AS runs_running,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL AND r.stand_kept)::int AS kept_stands,
       (SELECT count(*) FROM provider_profiles p WHERE p.tenant_id = @tenant_id AND p.deleted_at IS NULL)::int AS providers,
       (SELECT max(a.at) FROM audit_log a WHERE a.tenant_id = @tenant_id) AS last_activity_at;

-- name: AdminProfiles :many
SELECT p.id, p.email, p.display_name, p.avatar, p.is_platform_admin, p.preferences, p.notifications, p.created_at, p.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.user_id = p.id)::int AS memberships,
       (SELECT t.id FROM tenants t WHERE t.owner_id = p.id AND t.deleted_at IS NULL LIMIT 1) AS owned_tenant_id,
       (SELECT t.name FROM tenants t WHERE t.owner_id = p.id AND t.deleted_at IS NULL LIMIT 1) AS owned_tenant_name,
       la.last_activity_at
FROM profiles p,
     -- last activity: the newest audit entry the user acted in (as user or admin).
     LATERAL (SELECT max(a.at) AS last_activity_at FROM audit_log a WHERE a.actor_kind IN ('user', 'admin') AND a.actor_id = p.id::text) la
WHERE (@search::text = '' OR p.email ILIKE '%' || @search::text || '%' OR p.display_name ILIKE '%' || @search::text || '%')
  AND (NOT @only_admins::boolean OR p.is_platform_admin)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN p.display_name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN p.display_name END DESC,
  CASE WHEN @sort_key::text = 'email' AND NOT @desc::boolean THEN p.email END ASC,
  CASE WHEN @sort_key::text = 'email' AND @desc::boolean THEN p.email END DESC,
  CASE WHEN @sort_key::text = 'platform_admin' AND NOT @desc::boolean THEN p.is_platform_admin END ASC,
  CASE WHEN @sort_key::text = 'platform_admin' AND @desc::boolean THEN p.is_platform_admin END DESC,
  CASE WHEN @sort_key::text = 'memberships' AND NOT @desc::boolean THEN (SELECT count(*) FROM tenant_members m WHERE m.user_id = p.id) END ASC,
  CASE WHEN @sort_key::text = 'memberships' AND @desc::boolean THEN (SELECT count(*) FROM tenant_members m WHERE m.user_id = p.id) END DESC,
  CASE WHEN @sort_key::text = 'last_activity' AND NOT @desc::boolean THEN la.last_activity_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_activity' AND @desc::boolean THEN la.last_activity_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN p.created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN p.created_at END DESC,
  p.created_at DESC, p.id
LIMIT @lim OFFSET @off;

-- name: SetPlatformAdmin :execrows
UPDATE profiles SET is_platform_admin = @is_platform_admin, updated_at = now() WHERE id = @id;

-- name: AdminRuns :many
SELECT r.id, r.tenant_id, r.name, r.status, r.phase, r.status_reason, r.trigger, r.suite_run_id, r.cell_id, r.schedule_id, r.parent_run_id, r.test_id, r.test_name, r.author_id,
       r.snapshot, r.run_spec, r.summary, r.result, r.runtime_state, r.last_event_id, r.rating_tenant, r.rating_global, r.keep, r.keep_until, r.stand_kept, r.notes, r.labels,
       r.graphene_namespace, r.graphene_run_id, r.pipeline_revision, r.tps, r.duration_seconds, r.created_at, r.started_at, r.finished_at, r.updated_at, r.deleted_at,
       t.slug AS tenant_slug, t.name AS tenant_name
FROM runs r JOIN tenants t ON t.id = r.tenant_id
WHERE r.deleted_at IS NULL
  AND (COALESCE(cardinality(@statuses::text[]), 0) = 0 OR r.status = ANY(@statuses::text[]))
  AND (@tenant::text = '' OR t.slug = @tenant::text)
ORDER BY
  -- default: live runs (pending, running, cancelling), then the newest.
  CASE WHEN @sort_key::text = 'default' THEN r.status IN ('pending', 'running', 'cancelling') END DESC,
  CASE WHEN @sort_key::text = 'tenant' AND NOT @desc::boolean THEN t.slug END ASC,
  CASE WHEN @sort_key::text = 'tenant' AND @desc::boolean THEN t.slug END DESC,
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN r.name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN r.name END DESC,
  CASE WHEN @sort_key::text = 'status' AND NOT @desc::boolean THEN r.status END ASC,
  CASE WHEN @sort_key::text = 'status' AND @desc::boolean THEN r.status END DESC,
  CASE WHEN @sort_key::text = 'trigger' AND NOT @desc::boolean THEN r.trigger END ASC,
  CASE WHEN @sort_key::text = 'trigger' AND @desc::boolean THEN r.trigger END DESC,
  CASE WHEN @sort_key::text = 'db_kind' AND NOT @desc::boolean THEN r.summary->>'db_kind' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'db_kind' AND @desc::boolean THEN r.summary->>'db_kind' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND NOT @desc::boolean THEN r.summary->>'workload_name' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND @desc::boolean THEN r.summary->>'workload_name' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'topology' AND NOT @desc::boolean THEN r.summary->>'topology_label' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'topology' AND @desc::boolean THEN r.summary->>'topology_label' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND NOT @desc::boolean THEN r.summary->'provider_profile'->>'name' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND @desc::boolean THEN r.summary->'provider_profile'->>'name' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN r.author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN r.author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'tps' AND NOT @desc::boolean THEN r.tps END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'tps' AND @desc::boolean THEN r.tps END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'p99' AND NOT @desc::boolean THEN (r.summary->'headline'->>'latency_p99_ms')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'p99' AND @desc::boolean THEN (r.summary->'headline'->>'latency_p99_ms')::float8 END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'p50' AND NOT @desc::boolean THEN (r.summary->'headline'->>'latency_p50_ms')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'p50' AND @desc::boolean THEN (r.summary->'headline'->>'latency_p50_ms')::float8 END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'qps' AND NOT @desc::boolean THEN (r.summary->'headline'->>'qps')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'qps' AND @desc::boolean THEN (r.summary->'headline'->>'qps')::float8 END DESC NULLS LAST,
  -- A run without errors in its headline had none: it sorts as 0.
  CASE WHEN @sort_key::text = 'errors' AND NOT @desc::boolean THEN COALESCE((r.summary->'headline'->>'errors')::float8, 0) END ASC,
  CASE WHEN @sort_key::text = 'errors' AND @desc::boolean THEN COALESCE((r.summary->'headline'->>'errors')::float8, 0) END DESC,
  CASE WHEN @sort_key::text = 'duration' AND NOT @desc::boolean THEN r.duration_seconds END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'duration' AND @desc::boolean THEN r.duration_seconds END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'started_at' AND NOT @desc::boolean THEN r.started_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'started_at' AND @desc::boolean THEN r.started_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'finished_at' AND NOT @desc::boolean THEN r.finished_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'finished_at' AND @desc::boolean THEN r.finished_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN r.updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN r.updated_at END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN r.created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN r.created_at END DESC,
  r.created_at DESC, r.id
LIMIT @lim OFFSET @off;

-- name: AdminRunCounts :one
SELECT count(*) FILTER (WHERE status IN ('running', 'cancelling'))::int AS running,
       count(*) FILTER (WHERE status = 'pending')::int AS pending,
       count(*) FILTER (WHERE stand_kept)::int AS kept_stands
FROM runs WHERE deleted_at IS NULL;

-- name: AuditAll :many
SELECT a.id, a.at, a.tenant_id, a.actor_kind, a.actor_id, a.actor_name, a.action, a.target_kind, a.target_id, a.target_name, a.details, a.request_id
FROM audit_log a LEFT JOIN tenants t ON t.id = a.tenant_id
WHERE (@before_id::bigint = 0 OR a.id < @before_id::bigint)
  AND (@tenant::text = '' OR t.slug = @tenant::text)
  AND (@action::text = '' OR a.action = @action::text)
  AND (@actor_id::text = '' OR a.actor_id = @actor_id::text)
  AND (@since::timestamptz IS NULL OR a.at >= @since::timestamptz)
ORDER BY a.id DESC LIMIT @lim;

-- name: SystemSettingsRow :one
INSERT INTO system_settings (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET id = 1
RETURNING id, tenant_creation, public_rating_enabled, examples_enabled, default_limits, run_retention_max_days, stroppy_catalog, updated_at, updated_by;

-- name: UpdateSystemSettings :one
UPDATE system_settings
SET tenant_creation        = COALESCE(@tenant_creation::text, tenant_creation),
    public_rating_enabled  = COALESCE(@public_rating_enabled::boolean, public_rating_enabled),
    examples_enabled       = COALESCE(@examples_enabled::boolean, examples_enabled),
    default_limits         = CASE WHEN @set_default_limits::boolean THEN @default_limits::jsonb ELSE default_limits END,
    run_retention_max_days = COALESCE(@run_retention_max_days::int, run_retention_max_days),
    stroppy_catalog        = CASE WHEN @set_catalog::boolean THEN @stroppy_catalog::jsonb ELSE stroppy_catalog END,
    updated_at             = now(),
    updated_by             = @updated_by
WHERE id = 1
RETURNING id, tenant_creation, public_rating_enabled, examples_enabled, default_limits, run_retention_max_days, stroppy_catalog, updated_at, updated_by;
