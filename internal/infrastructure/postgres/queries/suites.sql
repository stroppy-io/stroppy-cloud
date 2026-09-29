-- name: InsertSuite :exec
INSERT INTO suites (id, tenant_id, name, description, tags, author_id, tests, axes, cells, concurrency, defaults)
VALUES (@id, @tenant_id, @name, @description, @tags, @author_id, @tests, @axes, @cells, @concurrency, @defaults);

-- name: SuiteByID :one
SELECT id, tenant_id, name, description, tags, author_id, tests, axes, cells, concurrency, defaults, created_at, updated_at
FROM suites WHERE id = @id AND deleted_at IS NULL;

-- name: SuiteByName :one
SELECT id FROM suites WHERE tenant_id = @tenant_id AND name = @name AND deleted_at IS NULL;

-- name: SuitesOfTenant :many
SELECT id, tenant_id, name, description, tags, author_id, tests, axes, cells, concurrency, defaults, created_at, updated_at
FROM suites
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR description ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
  AND (@favorites_of::text = '' OR EXISTS (SELECT 1 FROM favorites f WHERE f.user_id::text = @favorites_of::text AND f.kind = 'suite' AND f.target_id = suites.id))
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'test_count' AND NOT @desc::boolean THEN jsonb_array_length(tests) END ASC,
  CASE WHEN @sort_key::text = 'test_count' AND @desc::boolean THEN jsonb_array_length(tests) END DESC,
  CASE WHEN @sort_key::text = 'cell_count' AND NOT @desc::boolean THEN jsonb_array_length(cells) END ASC,
  CASE WHEN @sort_key::text = 'cell_count' AND @desc::boolean THEN jsonb_array_length(cells) END DESC,
  CASE WHEN @sort_key::text = 'last_run_at' AND NOT @desc::boolean THEN (SELECT max(sr.created_at) FROM suite_runs sr WHERE sr.suite_id = suites.id AND sr.deleted_at IS NULL) END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_run_at' AND @desc::boolean THEN (SELECT max(sr.created_at) FROM suite_runs sr WHERE sr.suite_id = suites.id AND sr.deleted_at IS NULL) END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'schedules' AND NOT @desc::boolean THEN (SELECT count(*) FROM schedules sc WHERE sc.target_kind = 'suite' AND sc.target_id = suites.id AND sc.deleted_at IS NULL) END ASC,
  CASE WHEN @sort_key::text = 'schedules' AND @desc::boolean THEN (SELECT count(*) FROM schedules sc WHERE sc.target_kind = 'suite' AND sc.target_id = suites.id AND sc.deleted_at IS NULL) END DESC,
  CASE WHEN @sort_key::text = 'next_run_at' AND NOT @desc::boolean THEN (SELECT min(sc.next_run_at) FROM schedules sc WHERE sc.target_kind = 'suite' AND sc.target_id = suites.id AND sc.deleted_at IS NULL AND sc.enabled) END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'next_run_at' AND @desc::boolean THEN (SELECT min(sc.next_run_at) FROM schedules sc WHERE sc.target_kind = 'suite' AND sc.target_id = suites.id AND sc.deleted_at IS NULL AND sc.enabled) END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  updated_at DESC, id
LIMIT @lim OFFSET @off;

-- name: UpdateSuite :execrows
UPDATE suites
SET name        = COALESCE(@name::text, name),
    description = COALESCE(@description::text, description),
    tags        = COALESCE(@tags::jsonb, tags),
    tests       = COALESCE(@tests::jsonb, tests),
    axes        = COALESCE(@axes::jsonb, axes),
    cells       = COALESCE(@cells::jsonb, cells),
    concurrency = COALESCE(@concurrency::int, concurrency),
    defaults    = COALESCE(@defaults::jsonb, defaults),
    updated_at  = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteSuite :execrows
UPDATE suites SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: SuitesUsingTest :many
SELECT id, name FROM suites WHERE deleted_at IS NULL AND tests @> jsonb_build_array(jsonb_build_object('ref', @test_id::text)) ORDER BY name;

-- name: InsertSuiteRun :exec
INSERT INTO suite_runs (id, tenant_id, suite_id, suite_name, name, status, trigger, schedule_id, retry_of, concurrency, cells, labels, author_id, graphene_namespace, idempotency_key)
VALUES (@id, @tenant_id, @suite_id, @suite_name, @name, @status, @trigger, @schedule_id, @retry_of, @concurrency, @cells, @labels, @author_id, @graphene_namespace, @idempotency_key);

-- name: SuiteRunByID :one
SELECT id, tenant_id, suite_id, suite_name, name, status, status_reason, trigger, schedule_id, retry_of, concurrency, cells, labels, author_id,
       graphene_namespace, last_event_id, created_at, started_at, finished_at, duration_seconds, updated_at, deleted_at
FROM suite_runs WHERE id = @id;

-- name: SuiteRunByIdempotencyKey :one
SELECT id FROM suite_runs WHERE tenant_id = @tenant_id AND idempotency_key = @key AND deleted_at IS NULL;

-- name: SuiteRunsOfTenant :many
SELECT id, tenant_id, suite_id, suite_name, name, status, status_reason, trigger, schedule_id, retry_of, concurrency, cells, labels, author_id,
       graphene_namespace, last_event_id, created_at, started_at, finished_at, duration_seconds, updated_at, deleted_at
FROM suite_runs
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (COALESCE(cardinality(@statuses::text[]), 0) = 0 OR status = ANY(@statuses::text[]))
  AND (COALESCE(cardinality(@triggers::text[]), 0) = 0 OR trigger = ANY(@triggers::text[]))
  AND (@suite_id::text = '' OR suite_id::text = @suite_id::text)
  AND (@started_after::timestamptz < '1970-01-02'::timestamptz OR started_at >= @started_after::timestamptz)
  AND (@started_before::timestamptz < '1970-01-02'::timestamptz OR started_at <= @started_before::timestamptz)
ORDER BY
  CASE WHEN @sort_key::text = 'started_at' AND NOT @desc::boolean THEN started_at END ASC,
  CASE WHEN @sort_key::text = 'started_at' AND @desc::boolean THEN started_at END DESC,
  CASE WHEN @sort_key::text = 'finished_at' AND NOT @desc::boolean THEN finished_at END ASC,
  CASE WHEN @sort_key::text = 'finished_at' AND @desc::boolean THEN finished_at END DESC,
  CASE WHEN @sort_key::text = 'status' AND NOT @desc::boolean THEN status END ASC,
  CASE WHEN @sort_key::text = 'status' AND @desc::boolean THEN status END DESC,
  CASE WHEN NOT @desc::boolean THEN created_at END ASC,
  created_at DESC
LIMIT @lim OFFSET @off;

-- name: SuiteRunsOfSuite :many
SELECT id, tenant_id, suite_id, suite_name, name, status, status_reason, trigger, schedule_id, retry_of, concurrency, cells, labels, author_id,
       graphene_namespace, last_event_id, created_at, started_at, finished_at, duration_seconds, updated_at, deleted_at
FROM suite_runs WHERE suite_id = @suite_id AND deleted_at IS NULL ORDER BY created_at DESC LIMIT @lim OFFSET @off;

-- name: SuiteRunCountOfSuite :one
SELECT count(*)::int AS n FROM suite_runs WHERE suite_id = @suite_id AND deleted_at IS NULL;

-- name: LiveSuiteRuns :many
SELECT id, tenant_id, graphene_namespace, last_event_id, status
FROM suite_runs WHERE deleted_at IS NULL AND status IN ('pending', 'running', 'cancelling') ORDER BY created_at;

-- name: SetSuiteRunStatus :exec
UPDATE suite_runs
SET status = CASE WHEN status = 'cancelling' AND @status IN ('pending', 'running') THEN status ELSE @status END,
    status_reason = CASE WHEN status = 'cancelling' AND @status IN ('pending', 'running') THEN status_reason ELSE @status_reason END,
    started_at  = COALESCE(started_at, @started_at),
    finished_at = COALESCE(@finished_at, finished_at),
    duration_seconds = CASE WHEN @finished_at IS NOT NULL AND started_at IS NOT NULL THEN EXTRACT(EPOCH FROM (@finished_at - started_at)) ELSE duration_seconds END,
    updated_at = now()
WHERE id = @id AND status NOT IN ('completed', 'failed', 'cancelled');

-- name: SetSuiteRunEvent :exec
UPDATE suite_runs SET last_event_id = GREATEST(last_event_id, @last_event_id), updated_at = now() WHERE id = @id;

-- name: SoftDeleteSuiteRun :execrows
UPDATE suite_runs SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: InsertSchedule :exec
INSERT INTO schedules (id, tenant_id, name, target_kind, target_id, target_name, cron, timezone, enabled, overrides, next_run_at, author_id)
VALUES (@id, @tenant_id, @name, @target_kind, @target_id, @target_name, @cron, @timezone, @enabled, @overrides, @next_run_at, @author_id);

-- name: ScheduleByID :one
SELECT id, tenant_id, name, target_kind, target_id, target_name, cron, timezone, enabled, overrides, next_run_at, last_run, author_id, created_at, updated_at
FROM schedules WHERE id = @id AND deleted_at IS NULL;

-- name: SchedulesOfTenant :many
SELECT id, tenant_id, name, target_kind, target_id, target_name, cron, timezone, enabled, overrides, next_run_at, last_run, author_id, created_at, updated_at
FROM schedules
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@target_kind::text = '' OR target_kind = @target_kind::text)
  AND (NOT @only_enabled::boolean OR enabled)
  AND (NOT @only_disabled::boolean OR NOT enabled)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'enabled' AND NOT @desc::boolean THEN enabled END ASC,
  CASE WHEN @sort_key::text = 'enabled' AND @desc::boolean THEN enabled END DESC,
  CASE WHEN @sort_key::text = 'target' AND NOT @desc::boolean THEN target_name END ASC,
  CASE WHEN @sort_key::text = 'target' AND @desc::boolean THEN target_name END DESC,
  CASE WHEN @sort_key::text = 'cron' AND NOT @desc::boolean THEN cron END ASC,
  CASE WHEN @sort_key::text = 'cron' AND @desc::boolean THEN cron END DESC,
  CASE WHEN @sort_key::text = 'next_run_at' AND NOT @desc::boolean THEN next_run_at END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'next_run_at' AND @desc::boolean THEN next_run_at END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_run_at' AND NOT @desc::boolean THEN (last_run->>'at')::timestamptz END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_run_at' AND @desc::boolean THEN (last_run->>'at')::timestamptz END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  created_at DESC, id
LIMIT @lim OFFSET @off;

-- name: SchedulesOfTarget :many
SELECT id, name FROM schedules WHERE target_kind = @target_kind AND target_id = @target_id AND deleted_at IS NULL ORDER BY name;

-- name: DueSchedules :many
SELECT id, tenant_id, name, target_kind, target_id, target_name, cron, timezone, enabled, overrides, next_run_at, last_run, author_id, created_at, updated_at
FROM schedules WHERE deleted_at IS NULL AND enabled AND next_run_at IS NOT NULL AND next_run_at <= @now ORDER BY next_run_at LIMIT 100;

-- name: UpdateSchedule :execrows
UPDATE schedules
SET name        = COALESCE(@name::text, name),
    target_kind = COALESCE(@target_kind::text, target_kind),
    target_id   = COALESCE(@target_id::uuid, target_id),
    target_name = COALESCE(@target_name::text, target_name),
    cron        = COALESCE(@cron::text, cron),
    timezone    = COALESCE(@timezone::text, timezone),
    enabled     = COALESCE(@enabled::boolean, enabled),
    overrides   = COALESCE(@overrides::jsonb, overrides),
    next_run_at = @next_run_at,
    updated_at  = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetScheduleFired :exec
UPDATE schedules SET next_run_at = @next_run_at, last_run = @last_run, updated_at = now() WHERE id = @id;

-- name: SoftDeleteSchedule :execrows
UPDATE schedules SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: InsertScheduleHistory :exec
INSERT INTO schedule_history (schedule_id, kind, ref_id, name, status, error) VALUES (@schedule_id, @kind, @ref_id, @name, @status, @error);

-- name: ScheduleHistory :many
SELECT id, schedule_id, kind, ref_id, name, status, error, at FROM schedule_history WHERE schedule_id = @schedule_id ORDER BY at DESC LIMIT @lim OFFSET @off;

-- name: UpcomingSchedules :many
SELECT id, tenant_id, name, target_kind, target_id, target_name, cron, timezone, enabled, overrides, next_run_at, last_run, author_id, created_at, updated_at
FROM schedules WHERE tenant_id = @tenant_id AND deleted_at IS NULL AND enabled AND next_run_at IS NOT NULL ORDER BY next_run_at LIMIT @lim;
