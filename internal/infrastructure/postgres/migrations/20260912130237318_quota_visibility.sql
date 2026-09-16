-- sqld:up
ALTER TABLE "public"."provider_profiles" ADD COLUMN "quotas_scope" text NOT NULL DEFAULT ''::text;
ALTER TABLE "public"."provider_profiles" ADD COLUMN "quotas_unavailable_reason" text NOT NULL DEFAULT ''::text;

-- sqld:down
ALTER TABLE "public"."provider_profiles" DROP COLUMN "quotas_unavailable_reason";
ALTER TABLE "public"."provider_profiles" DROP COLUMN "quotas_scope";
