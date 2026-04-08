ALTER TABLE auth_sessions
ADD COLUMN IF NOT EXISTS prev_refresh_token_hash TEXT,
ADD COLUMN IF NOT EXISTS compromised_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS ix_auth_sessions_tenant_prev_refresh
ON auth_sessions (tenant_id, prev_refresh_token_hash)
WHERE prev_refresh_token_hash IS NOT NULL;
