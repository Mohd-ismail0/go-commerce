ALTER TABLE auth_sessions
ADD COLUMN IF NOT EXISTS device_id TEXT,
ADD COLUMN IF NOT EXISTS ip_hash TEXT,
ADD COLUMN IF NOT EXISTS user_agent TEXT;

CREATE INDEX IF NOT EXISTS ix_auth_sessions_tenant_user_device
ON auth_sessions (tenant_id, user_id, device_id);
