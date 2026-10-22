-- sqld:up
ALTER TABLE "public"."provider_profiles" ADD COLUMN "config_revision" int4 NOT NULL DEFAULT 0;

-- sqld:down
ALTER TABLE "public"."provider_profiles" DROP COLUMN "config_revision";
