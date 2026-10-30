-- sqld:up
-- The catalog offers Stroppy 6.3.0 alone; workloads pinned to an earlier
-- release move to it (6.3 parameters are a superset of 6.0-6.2).
UPDATE "public"."workloads"
SET "stroppy_version" = '6.3.0',
    "spec" = CASE WHEN "spec" ? 'stroppy_version' THEN jsonb_set("spec", '{stroppy_version}', '"6.3.0"') ELSE "spec" END,
    "updated_at" = now()
WHERE "stroppy_version" <> '6.3.0';
UPDATE "public"."tests"
SET "workload_inline" = jsonb_set("workload_inline", '{stroppy_version}', '"6.3.0"'),
    "updated_at" = now()
WHERE "workload_inline" ? 'stroppy_version' AND "workload_inline"->>'stroppy_version' <> '6.3.0';

-- sqld:down
-- The earlier releases are gone from the catalog; nothing to restore.
