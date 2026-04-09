CREATE TABLE IF NOT EXISTS variant_channel_listings (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  variant_id TEXT NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  currency TEXT NOT NULL,
  price_cents BIGINT NOT NULL,
  is_published BOOLEAN NOT NULL DEFAULT FALSE,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, variant_id, channel_id)
);

CREATE INDEX IF NOT EXISTS ix_vcl_channel ON variant_channel_listings (tenant_id, region_id, channel_id);
CREATE INDEX IF NOT EXISTS ix_vcl_variant ON variant_channel_listings (tenant_id, region_id, variant_id);
