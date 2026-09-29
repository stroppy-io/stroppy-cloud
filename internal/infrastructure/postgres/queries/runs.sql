-- name: InsertRun :exec
INSERT INTO runs (id, tenant_id, name, status, phase, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
                  snapshot, run_spec, summary, rating_tenant, rating_global, keep, notes, labels, graphene_namespace, graphene_run_id, pipeline_revision, idempotency_key)
VALUES (@id, @tenant_id, @name, @status, @phase, @trigger, @suite_run_id, @cell_id, @schedule_id, @parent_run_id, @test_id, @test_name, @author_id,
        @snapshot, @run_spec, @summary, @rating_tenant, @rating_global, @keep, @notes, @labels, @graphene_namespace, @graphene_run_id, @pipeline_revision, @idempotency_key);

-- name: RunByID :one
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs WHERE id = @id;

-- name: RunByIdempotencyKey :one
SELECT id FROM runs WHERE tenant_id = @tenant_id AND idempotency_key = @key AND deleted_at IS NULL;

-- name: RunsOfTenant :many
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR test_name ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (cardinality(@statuses::text[]) = 0 OR status = ANY(@statuses::text[]))
  AND (cardinality(@kinds::text[]) = 0 OR summary->>'db_kind' = ANY(@kinds::text[]))
  AND (cardinality(@profiles::text[]) = 0 OR summary->'provider_profile'->>'id' = ANY(@profiles::text[]))
  AND (cardinality(@triggers::text[]) = 0 OR trigger = ANY(@triggers::text[]))
  AND (@test_id::text = '' OR test_id::text = @test_id::text)
  AND (@suite_run_id::text = '' OR suite_run_id::text = @suite_run_id::text)
  AND (NOT @standalone::boolean OR suite_run_id IS NULL)
  AND (@labels::jsonb IS NULL OR labels @> @labels::jsonb)
  AND (@started_after::timestamptz <= '1970-01-02'::timestamptz OR started_at >= @started_after::timestamptz)
  AND (@started_before::timestamptz <= '1970-01-02'::timestamptz OR started_at <= @started_before::timestamptz)
  AND (@finished_after::timestamptz <= '1970-01-02'::timestamptz OR finished_at >= @finished_after::timestamptz)
  AND (@finished_before::timestamptz <= '1970-01-02'::timestamptz OR finished_at <= @finished_before::timestamptz)
  AND (@duration_min::double precision = 0 OR duration_seconds >= @duration_min::double precision)
  AND (@duration_max::double precision = 0 OR duration_seconds <= @duration_max::double precision)
  AND (@stand_kept_only::boolean = false OR stand_kept)
  AND (@favorites_of::text = '' OR EXISTS (SELECT 1 FROM favorites f WHERE f.kind = 'run' AND f.target_id = runs.id AND f.user_id::text = @favorites_of::text))
ORDER BY
  -- default: the viewer's favorites, then live runs, then the newest.
  CASE WHEN @sort_key::text = 'default' THEN EXISTS (SELECT 1 FROM favorites f WHERE f.kind = 'run' AND f.target_id = runs.id AND f.user_id::text = @viewer_id::text) END DESC,
  CASE WHEN @sort_key::text = 'default' THEN status IN ('pending', 'running', 'cancelling') END DESC,
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'status' AND NOT @desc::boolean THEN status END ASC,
  CASE WHEN @sort_key::text = 'status' AND @desc::boolean THEN status END DESC,
  CASE WHEN @sort_key::text = 'trigger' AND NOT @desc::boolean THEN trigger END ASC,
  CASE WHEN @sort_key::text = 'trigger' AND @desc::boolean THEN trigger END DESC,
  CASE WHEN @sort_key::text = 'db_kind' AND NOT @desc::boolean THEN summary->>'db_kind' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'db_kind' AND @desc::boolean THEN summary->>'db_kind' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND NOT @desc::boolean THEN summary->>'workload_name' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND @desc::boolean THEN summary->>'workload_name' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'topology' AND NOT @desc::boolean THEN summary->>'topology_label' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'topology' AND @desc::boolean THEN summary->>'topology_label' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND NOT @desc::boolean THEN summary->'provider_profile'->>'name' END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND @desc::boolean THEN summary->'provider_profile'->>'name' END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'tps' AND NOT @desc::boolean THEN tps END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'tps' AND @desc::boolean THEN tps END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'p99' AND NOT @desc::boolean THEN (summary->'headline'->>'latency_p99_ms')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'p99' AND @desc::boolean THEN (summary->'headline'->>'latency_p99_ms')::float8 END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'p50' AND NOT @desc::boolean THEN (summary->'headline'->>'latency_p50_ms')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'p50' AND @desc::boolean THEN (summary->'headline'->>'latency_p50_ms')::float8 END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'qps' AND NOT @desc::boolean THEN (summary->'headline'->>'qps')::float8 END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'qps' AND @desc::boolean THEN (summary->'headline'->>'qps')::float8 END DESC NULLS LAST,
  -- A run without errors in its headline had none: it sorts as 0.
  CASE WHEN @sort_key::text = 'errors' AND NOT @desc::boolean THEN COALESCE((summary->'headline'->>'errors')::float8, 0) END ASC,
  CASE WHEN @sort_key::text = 'errors' AND @desc::boolean THEN COALESCE((summary->'headline'->>'errors')::float8, 0) END DESC,
  CASE WHEN @sort_key::text = 'duration' AND NOT @desc::boolean THEN duration_seconds END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'duration' AND @desc::boolean THEN duration_seconds END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'started_at' AND NOT @desc::boolean THEN started_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'started_at' AND @desc::boolean THEN started_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'finished_at' AND NOT @desc::boolean THEN finished_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'finished_at' AND @desc::boolean THEN finished_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  created_at DESC, id
LIMIT @lim OFFSET @off;

-- name: RunFacetValues :many
-- One row per (field, value): status, db_kind, trigger, provider profile, author.
SELECT field, value, count(*)::int AS count FROM (
  SELECT 'status' AS field, status AS value FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  UNION ALL SELECT 'kind', summary->>'db_kind' FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND summary ? 'db_kind'
  UNION ALL SELECT 'trigger', trigger FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  UNION ALL SELECT 'provider_profile', summary->'provider_profile'->>'id' FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND summary ? 'provider_profile'
  UNION ALL SELECT 'author', author_id::text FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND author_id IS NOT NULL
) f GROUP BY field, value ORDER BY field, count DESC, value;

-- name: RunsOfTest :many
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs WHERE test_id = @test_id AND deleted_at IS NULL
ORDER BY created_at DESC LIMIT @lim OFFSET @off;

-- name: LiveRuns :many
-- Active workflows and terminal runs whose kept infrastructure still needs reconciliation.
SELECT id, tenant_id, graphene_namespace, last_event_id, status
FROM runs WHERE deleted_at IS NULL AND (status IN ('pending', 'running', 'cancelling') OR stand_kept) ORDER BY created_at;

-- name: RunsOfSuiteRun :many
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs WHERE suite_run_id = @suite_run_id AND deleted_at IS NULL ORDER BY cell_id;

-- name: RunsOfSchedule :many
SELECT id, name, status, created_at FROM runs WHERE schedule_id = @schedule_id AND deleted_at IS NULL ORDER BY created_at DESC LIMIT @lim;

-- name: TenantLiveRunCount :one
SELECT count(*)::int AS n FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND status IN ('pending', 'running', 'cancelling');

-- name: TenantHasLiveRuns :one
SELECT EXISTS (SELECT 1 FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND status IN ('pending', 'running', 'cancelling'))::boolean AS live;

-- name: UpdateRunMeta :execrows
UPDATE runs
SET name          = COALESCE(@name::text, name),
    notes         = COALESCE(@notes::text, notes),
    labels        = COALESCE(@labels::jsonb, labels),
    rating_tenant = COALESCE(@rating_tenant::boolean, rating_tenant),
    rating_global = COALESCE(@rating_global::boolean, rating_global),
    updated_at    = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetRunStatus :exec
UPDATE runs
SET status = CASE WHEN status = 'cancelling' AND @status IN ('pending', 'running') THEN status ELSE @status END,
    phase = @phase,
    status_reason = CASE WHEN status = 'cancelling' AND @status IN ('pending', 'running') THEN status_reason ELSE @status_reason END,
    started_at  = COALESCE(started_at, @started_at),
    finished_at = COALESCE(@finished_at, finished_at),
    duration_seconds = CASE WHEN @finished_at IS NOT NULL AND started_at IS NOT NULL THEN EXTRACT(EPOCH FROM (@finished_at - started_at)) ELSE duration_seconds END,
    updated_at = now()
WHERE id = @id AND status NOT IN ('completed', 'failed', 'cancelled');

-- name: SetRunProjection :exec
UPDATE runs SET runtime_state = @runtime_state, last_event_id = GREATEST(last_event_id, @last_event_id), updated_at = now() WHERE id = @id;

-- name: SetRunResult :exec
UPDATE runs SET result = @result, summary = @summary, tps = @tps, updated_at = now() WHERE id = @id;

-- name: RunsWithoutQPSSeries :many
-- Finished runs with a result whose throughput series was never stored (backfill).
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs
WHERE deleted_at IS NULL AND status IN ('completed', 'failed', 'cancelled') AND result IS NOT NULL AND NOT (summary ? 'qps_series')
ORDER BY finished_at DESC NULLS LAST
LIMIT @lim;

-- name: SetRunQPSSeries :exec
-- A derived field: updated_at stays, the run did not change.
UPDATE runs SET summary = jsonb_set(summary, '{qps_series}', @series::jsonb) WHERE id = @id;

-- name: SetRunKeep :exec
UPDATE runs SET stand_kept = @stand_kept, keep_until = @keep_until, updated_at = now() WHERE id = @id;

-- name: SoftDeleteRun :execrows
UPDATE runs SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: InsertRunEvent :execrows
INSERT INTO run_events (run_id, graphene_id, at, kind, title, subject, status, error, attempt, payload)
VALUES (@run_id, @graphene_id, @at, @kind, @title, @subject, @status, @error, @attempt, @payload)
ON CONFLICT DO NOTHING;

-- name: RunEventsAfter :many
SELECT id, run_id, graphene_id, at, kind, title, subject, status, error, attempt, payload
FROM run_events WHERE run_id = @run_id AND id > @after ORDER BY id LIMIT @lim;

-- name: SetFavorite :exec
INSERT INTO favorites (user_id, tenant_id, kind, target_id) VALUES (@user_id, @tenant_id, @kind, @target_id) ON CONFLICT DO NOTHING;

-- name: UnsetFavorite :execrows
DELETE FROM favorites WHERE user_id = @user_id AND kind = @kind AND target_id = @target_id;

-- name: FavoritesOf :many
SELECT target_id FROM favorites WHERE user_id = @user_id AND tenant_id = @tenant_id AND kind = @kind;

-- name: RatingRuns :many
-- Completed, opted-in runs ranked by a headline metric; league = the
-- sizes signature, so only equal hardware competes.
SELECT r.id, r.tenant_id, r.name, r.author_id, r.summary, r.finished_at, r.tps,
       t.name AS tenant_name, t.public_name AS tenant_public_name,
       (SELECT string_agg(key || '=' || value, ',' ORDER BY key) FROM jsonb_each_text(r.summary->'sizes')) AS league,
       (r.summary->'headline'->>@metric::text)::float8 AS value
FROM runs r JOIN tenants t ON t.id = r.tenant_id
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.summary->'headline' ? @metric::text
  AND coalesce(r.summary->>'db_kind', '') NOT IN ('', 'external', 'noop')
  AND ((@tenant_id::text <> '' AND r.tenant_id::text = @tenant_id::text AND r.rating_tenant) OR (@tenant_id::text = '' AND r.rating_global))
  AND (cardinality(@kinds::text[]) = 0 OR r.summary->>'db_kind' = ANY(@kinds::text[]))
  AND (cardinality(@versions::text[]) = 0 OR r.summary->>'db_version' = ANY(@versions::text[]))
  AND (cardinality(@providers::text[]) = 0 OR r.summary->>'provider_kind' = ANY(@providers::text[]))
  AND (cardinality(@stroppy_versions::text[]) = 0 OR r.summary->>'stroppy_version' = ANY(@stroppy_versions::text[]))
  AND (@league::text = '' OR (SELECT string_agg(key || '=' || value, ',' ORDER BY key) FROM jsonb_each_text(r.summary->'sizes')) = @league::text)
  AND (@since::timestamptz < '1970-01-02'::timestamptz OR r.finished_at >= @since::timestamptz)
ORDER BY CASE WHEN @higher_is_better::boolean THEN -(r.summary->'headline'->>@metric::text)::float8 ELSE (r.summary->'headline'->>@metric::text)::float8 END, r.finished_at DESC
LIMIT @lim OFFSET @off;

-- name: RatingLeagues :many
SELECT DISTINCT (SELECT string_agg(key || '=' || value, ',' ORDER BY key) FROM jsonb_each_text(summary->'sizes')) AS league
FROM runs WHERE deleted_at IS NULL AND status = 'completed'
  AND ((@tenant_id::text <> '' AND tenant_id::text = @tenant_id::text AND rating_tenant) OR (@tenant_id::text = '' AND rating_global))
ORDER BY 1;

-- name: RunCountsOfTenant :one
SELECT count(*)::int AS total,
       count(*) FILTER (WHERE status = 'pending')::int AS pending,
       count(*) FILTER (WHERE status IN ('running', 'cancelling'))::int AS running,
       count(*) FILTER (WHERE status = 'completed')::int AS completed,
       count(*) FILTER (WHERE status = 'failed')::int AS failed,
       count(*) FILTER (WHERE status = 'cancelled')::int AS cancelled,
       count(*) FILTER (WHERE stand_kept)::int AS kept_stands
FROM runs WHERE tenant_id = @tenant_id AND deleted_at IS NULL;

-- name: RunsByIDs :many
SELECT id, tenant_id, name, status, phase, status_reason, trigger, suite_run_id, cell_id, schedule_id, parent_run_id, test_id, test_name, author_id,
       snapshot, run_spec, summary, result, runtime_state, last_event_id, rating_tenant, rating_global, keep, keep_until, stand_kept, notes, labels,
       graphene_namespace, graphene_run_id, pipeline_revision, tps, duration_seconds, created_at, started_at, finished_at, updated_at, deleted_at
FROM runs WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- name: TestRunStats :many
SELECT DISTINCT ON (test_id) test_id, id, name, status, started_at, count(*) OVER (PARTITION BY test_id) AS run_count
FROM runs WHERE test_id = ANY(@test_ids::uuid[]) AND deleted_at IS NULL
ORDER BY test_id, created_at DESC;
