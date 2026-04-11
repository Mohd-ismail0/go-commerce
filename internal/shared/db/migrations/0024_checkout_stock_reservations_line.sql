-- Per-checkout-line stock holds (soft reservations) for oversell protection while the session is open.

ALTER TABLE stock_reservations
ADD COLUMN IF NOT EXISTS checkout_line_id TEXT;

-- Legacy rows without a line id are not used by the checkout upsert path; remove to avoid skewing availability.
DELETE FROM stock_reservations WHERE checkout_line_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_stock_reservations_checkout_line
ON stock_reservations (tenant_id, region_id, checkout_id, checkout_line_id);

CREATE INDEX IF NOT EXISTS ix_stock_reservations_tenant_region_stock_item
ON stock_reservations (tenant_id, region_id, stock_item_id);
