ALTER TABLE stock_items
ADD COLUMN IF NOT EXISTS warehouse_id TEXT REFERENCES warehouses(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_stock_items_tenant_region_variant_qty
ON stock_items (tenant_id, region_id, variant_id, quantity DESC)
WHERE variant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_stock_items_tenant_region_product_qty
ON stock_items (tenant_id, region_id, product_id, quantity DESC)
WHERE product_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS payment_reconciliation_actions (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  payment_id TEXT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
  issue TEXT NOT NULL,
  action_type TEXT NOT NULL,
  status TEXT NOT NULL,
  note TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS ix_recon_actions_tenant_region_status
ON payment_reconciliation_actions (tenant_id, region_id, status, created_at DESC);
