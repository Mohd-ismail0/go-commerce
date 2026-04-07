CREATE UNIQUE INDEX IF NOT EXISTS ux_products_tenant_region_sku
ON products (tenant_id, region_id, sku);

CREATE INDEX IF NOT EXISTS ix_products_tenant_region_created_at
ON products (tenant_id, region_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_orders_tenant_region_status_created_at
ON orders (tenant_id, region_id, status, created_at DESC);

ALTER TABLE orders
  ADD CONSTRAINT chk_orders_status
  CHECK (status IN ('created', 'confirmed', 'completed', 'cancelled'));

CREATE TABLE IF NOT EXISTS idempotency_keys (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_idem_tenant_scope_key
ON idempotency_keys (tenant_id, scope, idempotency_key);

CREATE TABLE IF NOT EXISTS order_status_audit (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT NOT NULL,
  previous_status TEXT NOT NULL,
  new_status TEXT NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
