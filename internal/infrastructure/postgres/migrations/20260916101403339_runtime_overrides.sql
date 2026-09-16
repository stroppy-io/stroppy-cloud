-- sqld:up
ALTER TABLE "public"."databases" ADD COLUMN "runtime" jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE "public"."tests" ADD COLUMN "execution" jsonb NOT NULL DEFAULT '{}'::jsonb;

-- sqld:down
ALTER TABLE "public"."tests" DROP COLUMN "execution";
ALTER TABLE "public"."databases" DROP COLUMN "runtime";
