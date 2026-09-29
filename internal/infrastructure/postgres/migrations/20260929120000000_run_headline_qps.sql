-- sqld:up
-- Backfill the run-level headline numbers the finish path now derives
-- (run.CompleteHeadline) from the per-segment `<segment>.<metric>` keys:
--   qps    = Σ(qps·measurement_seconds) / Σ(measurement_seconds) over the
--            segments with both (window > 0), else the mean segment qps;
--   errors = Σ terminal_errors_total (failed_iterations_total +
--            failed_queries_total for a segment without it), kept only when
--            larger than the stored errors (which cover the last segment).
-- Idempotent: qps is written only where it is missing, errors only where
-- the derived count is larger.
WITH kv AS (
    SELECT r.id, e.key, (e.value #>> '{}')::float8 AS value
    FROM runs r, jsonb_each(r.summary->'headline') e
    WHERE jsonb_typeof(r.summary->'headline') = 'object'
      AND jsonb_typeof(e.value) = 'number'
      AND e.key ~ '.\.(queries_per_second|measurement_seconds|terminal_errors_total|failed_iterations_total|failed_queries_total)$'
), seg AS (
    SELECT id,
           regexp_replace(key, '\.[^.]+$', '') AS segment,
           max(value) FILTER (WHERE key LIKE '%.queries_per_second')      AS qps,
           max(value) FILTER (WHERE key LIKE '%.measurement_seconds')     AS win,
           max(value) FILTER (WHERE key LIKE '%.terminal_errors_total')   AS terminal,
           max(value) FILTER (WHERE key LIKE '%.failed_iterations_total') AS failed_iter,
           max(value) FILTER (WHERE key LIKE '%.failed_queries_total')    AS failed_query
    FROM kv
    GROUP BY id, segment
), agg AS (
    SELECT id,
           COALESCE(
               sum(qps * win) FILTER (WHERE qps IS NOT NULL AND win > 0)
                   / NULLIF(sum(win) FILTER (WHERE qps IS NOT NULL AND win > 0), 0),
               avg(qps)
           ) AS qps,
           sum(COALESCE(terminal, COALESCE(failed_iter, 0) + COALESCE(failed_query, 0)))
               FILTER (WHERE terminal IS NOT NULL OR failed_iter IS NOT NULL OR failed_query IS NOT NULL) AS errors
    FROM seg
    GROUP BY id
)
UPDATE runs r
SET summary = jsonb_set(
    r.summary, '{headline}',
    (r.summary->'headline') || jsonb_strip_nulls(jsonb_build_object(
        'qps', CASE WHEN NOT (r.summary->'headline' ? 'qps') THEN to_jsonb(a.qps) END,
        'errors', CASE WHEN a.errors > COALESCE((r.summary->'headline'->>'errors')::float8, 0) THEN to_jsonb(a.errors) END
    ))
)
FROM agg a
WHERE a.id = r.id
  AND ((a.qps IS NOT NULL AND NOT (r.summary->'headline' ? 'qps'))
       OR a.errors > COALESCE((r.summary->'headline'->>'errors')::float8, 0));

-- sqld:down
-- The derived errors cannot be told apart from reported ones; only qps goes.
UPDATE runs SET summary = jsonb_set(summary, '{headline}', (summary->'headline') - 'qps')
WHERE jsonb_typeof(summary->'headline') = 'object' AND summary->'headline' ? 'qps';
