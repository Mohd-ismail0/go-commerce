-- Product types and Saleor-style catalog attributes (tenant/region scoped).

CREATE TABLE IF NOT EXISTS product_types (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, slug)
);

CREATE TABLE IF NOT EXISTS catalog_attributes (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  input_type TEXT NOT NULL CHECK (input_type IN ('text', 'number', 'boolean', 'select')),
  unit TEXT,
  allowed_values JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, slug)
);

CREATE TABLE IF NOT EXISTS product_type_attributes (
  product_type_id TEXT NOT NULL REFERENCES product_types(id) ON DELETE CASCADE,
  attribute_id TEXT NOT NULL REFERENCES catalog_attributes(id) ON DELETE CASCADE,
  sort_order INT NOT NULL DEFAULT 0,
  variant_only BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (product_type_id, attribute_id)
);

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS product_type_id TEXT REFERENCES product_types(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_products_tenant_region_product_type
  ON products (tenant_id, region_id, product_type_id)
  WHERE product_type_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS product_attribute_values (
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  attribute_id TEXT NOT NULL REFERENCES catalog_attributes(id) ON DELETE CASCADE,
  value_text TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (product_id, attribute_id)
);

CREATE TABLE IF NOT EXISTS variant_attribute_values (
  variant_id TEXT NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
  attribute_id TEXT NOT NULL REFERENCES catalog_attributes(id) ON DELETE CASCADE,
  value_text TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (variant_id, attribute_id)
);
