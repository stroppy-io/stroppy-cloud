-- sqld:up
ALTER TABLE "public"."databases" ADD COLUMN "topology" jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE "public"."databases" DROP COLUMN "topology_label";

-- sqld:down
-- WARNING: data loss on rollback
ALTER TABLE "public"."databases" ADD COLUMN "topology_label" text NOT NULL DEFAULT ''::text;
ALTER TABLE "public"."databases" DROP COLUMN "topology";
