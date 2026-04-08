ALTER TABLE checkout_sessions
ADD COLUMN IF NOT EXISTS voucher_code TEXT,
ADD COLUMN IF NOT EXISTS promotion_id TEXT REFERENCES promotions(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS tax_class_id TEXT REFERENCES tax_classes(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS country_code TEXT;

CREATE INDEX IF NOT EXISTS ix_checkout_sessions_tenant_region_status_updated
ON checkout_sessions (tenant_id, region_id, status, updated_at DESC);
