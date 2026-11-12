-- sqld:up
-- workload.tpch@6.4.0 defaulted refresh_set to 1, and stroppy refuses
-- refresh-set without refresh-functions, so every TPC-H segment compiled
-- since then failed on load. The schema no longer defaults it; stored run
-- specs drop it wherever refresh_functions is not on, so queued runs and
-- reruns start.
UPDATE "public"."runs" r
SET "run_spec" = jsonb_set(r."run_spec", '{workload,segments}', (
    SELECT jsonb_agg(
             CASE WHEN e.s->'workload' ? 'refresh_set'
                       AND coalesce(e.s->'workload'->>'refresh_functions', 'false') <> 'true'
                  THEN jsonb_set(e.s, '{workload}', (e.s->'workload') - 'refresh_set')
                  ELSE e.s END
             ORDER BY e.i)
    FROM jsonb_array_elements(r."run_spec"->'workload'->'segments') WITH ORDINALITY AS e(s, i)))
WHERE EXISTS (
    SELECT 1 FROM jsonb_array_elements(coalesce(r."run_spec"->'workload'->'segments', '[]'::jsonb)) AS s
    WHERE s->'workload' ? 'refresh_set'
      AND coalesce(s->'workload'->>'refresh_functions', 'false') <> 'true');

-- sqld:down
-- The value made the runs fail; nothing to restore.
