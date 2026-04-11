CREATE TABLE IF NOT EXISTS customer_addresses (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  is_default_shipping BOOLEAN NOT NULL DEFAULT FALSE,
  is_default_billing BOOLEAN NOT NULL DEFAULT FALSE,
  first_name TEXT NOT NULL,
  last_name TEXT NOT NULL,
  company TEXT,
  street_line_1 TEXT NOT NULL,
  street_line_2 TEXT,
  city TEXT NOT NULL,
  postal_code TEXT NOT NULL,
  country_code TEXT NOT NULL,
  phone TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ix_customer_addresses_tenant_region_customer
ON customer_addresses (tenant_id, region_id, customer_id);

CREATE INDEX IF NOT EXISTS ix_customer_addresses_tenant_region_customer_default_ship
ON customer_addresses (tenant_id, region_id, customer_id)
WHERE is_default_shipping = TRUE;

CREATE INDEX IF NOT EXISTS ix_customer_addresses_tenant_region_customer_default_bill
ON customer_addresses (tenant_id, region_id, customer_id)
WHERE is_default_billing = TRUE;
