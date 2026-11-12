-- sqld:up
-- The platform is hardware of a test (sizes.<role>.machine.instance_type),
-- not of the cloud account: provider.yandex.settings@1 no longer has
-- platform_id, and provisioning never read it. Stored profiles drop it so
-- they still bake under the strict schema.
UPDATE "public"."provider_profiles"
SET "settings" = "settings" - 'platform_id'
WHERE "kind" = 'yandex' AND "settings" ? 'platform_id';

-- sqld:down
-- The value was never used; nothing to restore.
