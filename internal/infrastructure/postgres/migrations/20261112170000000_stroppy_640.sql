-- sqld:up
-- The catalog offers Stroppy 6.4.0 alone; workloads pinned to an earlier
-- release move to it (6.4.0 adds parameters, the ones of 6.3.3 keep their
-- meaning). updated_at stays: the platform moved them, nothing the tenant
-- validated changed, so their tests must not turn stale.
UPDATE "public"."workloads"
SET "stroppy_version" = '6.4.0',
    "spec" = CASE WHEN "spec" ? 'stroppy_version' THEN jsonb_set("spec", '{stroppy_version}', '"6.4.0"') ELSE "spec" END
WHERE "stroppy_version" <> '6.4.0';
UPDATE "public"."tests"
SET "workload_inline" = jsonb_set("workload_inline", '{stroppy_version}', '"6.4.0"')
WHERE "workload_inline" ? 'stroppy_version' AND "workload_inline"->>'stroppy_version' <> '6.4.0';

-- sqld:down
-- The earlier releases are gone from the catalog; nothing to restore.
