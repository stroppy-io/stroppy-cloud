-- sqld:up
ALTER TABLE "public"."runs" ADD COLUMN "admitted_at" timestamptz;
ALTER TABLE "public"."runs" ADD COLUMN "queue" jsonb NOT NULL DEFAULT '{}'::jsonb;
-- Runs from before the queue were all started at launch.
UPDATE "public"."runs" SET "admitted_at" = COALESCE("started_at", "created_at");
CREATE INDEX "runs_queue_idx" ON "public"."runs" ("tenant_id", "created_at") WHERE ((deleted_at IS NULL) AND (status = 'pending'::text) AND (admitted_at IS NULL));

-- sqld:down
DROP INDEX "public"."runs_queue_idx";
ALTER TABLE "public"."runs" DROP COLUMN "queue";
ALTER TABLE "public"."runs" DROP COLUMN "admitted_at";
