ALTER TABLE stock_items
ADD COLUMN IF NOT EXISTS variant_id TEXT REFERENCES product_variants(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_stock_items_tenant_region_product
ON stock_items (tenant_id, region_id, product_id)
WHERE product_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_stock_items_tenant_region_variant
ON stock_items (tenant_id, region_id, variant_id)
WHERE variant_id IS NOT NULL;
