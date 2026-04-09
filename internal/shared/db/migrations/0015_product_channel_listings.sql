CREATE TABLE IF NOT EXISTS product_channel_listings (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  is_published BOOLEAN NOT NULL DEFAULT FALSE,
  visible_in_listings BOOLEAN NOT NULL DEFAULT TRUE,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, product_id, channel_id)
);

CREATE INDEX IF NOT EXISTS ix_pcl_channel ON product_channel_listings (tenant_id, region_id, channel_id);
CREATE INDEX IF NOT EXISTS ix_pcl_product ON product_channel_listings (tenant_id, region_id, product_id);
