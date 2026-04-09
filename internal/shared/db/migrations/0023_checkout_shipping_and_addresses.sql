ALTER TABLE checkout_sessions
ADD COLUMN IF NOT EXISTS shipping_method_id TEXT,
ADD COLUMN IF NOT EXISTS shipping_address_country TEXT,
ADD COLUMN IF NOT EXISTS shipping_address_postal_code TEXT,
ADD COLUMN IF NOT EXISTS billing_address_country TEXT,
ADD COLUMN IF NOT EXISTS billing_address_postal_code TEXT;

CREATE INDEX IF NOT EXISTS ix_checkout_sessions_tenant_region_shipping_method_status
ON checkout_sessions (tenant_id, region_id, shipping_method_id, status, updated_at DESC);
