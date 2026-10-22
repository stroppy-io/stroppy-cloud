-- sqld:up
ALTER TABLE "public"."provider_profiles" ADD COLUMN "upgrade_after" timestamptz;

-- sqld:down
ALTER TABLE "public"."provider_profiles" DROP COLUMN "upgrade_after";
