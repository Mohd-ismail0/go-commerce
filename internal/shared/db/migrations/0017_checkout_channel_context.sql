ALTER TABLE checkout_sessions
ADD COLUMN IF NOT EXISTS channel_id TEXT;

CREATE INDEX IF NOT EXISTS ix_checkout_sessions_tenant_region_channel_status
ON checkout_sessions (tenant_id, region_id, channel_id, status, updated_at DESC);
