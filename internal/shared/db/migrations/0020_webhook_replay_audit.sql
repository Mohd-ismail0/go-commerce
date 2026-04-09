CREATE TABLE IF NOT EXISTS webhook_replay_audit (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  outbox_id TEXT NOT NULL REFERENCES event_outbox(id) ON DELETE CASCADE,
  reason TEXT NOT NULL,
  requested_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ix_webhook_replay_audit_outbox_created
ON webhook_replay_audit (tenant_id, region_id, outbox_id, created_at DESC);
