-- sqld:up
ALTER TABLE tenants ADD COLUMN retiring boolean NOT NULL DEFAULT false;
UPDATE provider_profiles SET status='failed', status_reason='provider configuration upgrade required: verify this profile before launching' WHERE deleted_at IS NULL AND status='ready';

-- sqld:down
ALTER TABLE tenants DROP COLUMN retiring;
