-- sqld:up
ALTER TABLE "public"."system_settings" DROP COLUMN "default_limits";
ALTER TABLE "public"."system_settings" DROP COLUMN "run_retention_max_days";
ALTER TABLE "public"."tenant_settings" DROP COLUMN "limits_override";

-- sqld:down
-- WARNING: data loss on rollback
ALTER TABLE "public"."tenant_settings" ADD COLUMN "limits_override" jsonb;
-- WARNING: data loss on rollback
ALTER TABLE "public"."system_settings" ADD COLUMN "run_retention_max_days" int4 NOT NULL DEFAULT 365;
-- WARNING: data loss on rollback
ALTER TABLE "public"."system_settings" ADD COLUMN "default_limits" jsonb;
