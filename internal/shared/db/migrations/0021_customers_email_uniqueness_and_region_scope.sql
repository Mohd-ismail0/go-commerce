CREATE UNIQUE INDEX IF NOT EXISTS ux_customers_tenant_region_email_ci
ON customers (tenant_id, region_id, LOWER(email));

CREATE INDEX IF NOT EXISTS ix_customers_tenant_region_updated_at
ON customers (tenant_id, region_id, updated_at DESC);
