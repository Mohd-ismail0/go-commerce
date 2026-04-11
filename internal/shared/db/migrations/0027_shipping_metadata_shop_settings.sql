-- Shipping method presentation + optional weight surcharge (per kg) for /shipping/resolve quotes.
ALTER TABLE shipping_methods
  ADD COLUMN IF NOT EXISTS delivery_days_min INTEGER,
  ADD COLUMN IF NOT EXISTS delivery_days_max INTEGER,
  ADD COLUMN IF NOT EXISTS description TEXT,
  ADD COLUMN IF NOT EXISTS weight_surcharge_per_kg_cents BIGINT;

COMMENT ON COLUMN shipping_methods.weight_surcharge_per_kg_cents IS 'When set with POST /shipping/resolve total_weight_grams, quoted price adds (grams * surcharge) / 1000 to flat price_cents.';

-- Tenant/region storefront metadata (Saleor Shop/Site subset).
CREATE TABLE IF NOT EXISTS shop_settings (
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  domain TEXT NOT NULL DEFAULT '',
  support_email TEXT NOT NULL DEFAULT '',
  company_address TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, region_id)
);
