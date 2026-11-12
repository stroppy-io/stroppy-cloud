-- sqld:up
-- A run compiled before provider.yandex.settings@1 lost platform_id carries
-- it in its stored RunSpec; the pipeline refuses the unknown field, so a run
-- still waiting in the queue (and any rerun of an older one) would fail on
-- start. The value was never read: drop it from every stored spec.
UPDATE "public"."runs"
SET "run_spec" = "run_spec" #- '{provider,settings,platform_id}'
WHERE "run_spec" #> '{provider,settings}' ? 'platform_id';

-- sqld:down
-- The value was never used; nothing to restore.
