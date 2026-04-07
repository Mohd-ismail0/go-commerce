CREATE TABLE IF NOT EXISTS permissions (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  is_staff BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, email)
);

CREATE TABLE IF NOT EXISTS roles (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id TEXT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS product_types (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, name)
);

CREATE TABLE IF NOT EXISTS categories (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  parent_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, slug)
);

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS product_type_id TEXT REFERENCES product_types(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS product_variants (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  sku TEXT NOT NULL,
  name TEXT NOT NULL,
  price_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, sku)
);

CREATE TABLE IF NOT EXISTS product_media (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  media_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS collections (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, slug)
);

CREATE TABLE IF NOT EXISTS collection_products (
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  PRIMARY KEY (collection_id, product_id)
);

CREATE TABLE IF NOT EXISTS checkout_sessions (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  customer_id TEXT NOT NULL,
  status TEXT NOT NULL,
  currency TEXT NOT NULL,
  subtotal_cents BIGINT NOT NULL DEFAULT 0,
  shipping_cents BIGINT NOT NULL DEFAULT 0,
  tax_cents BIGINT NOT NULL DEFAULT 0,
  total_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS checkout_lines (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  checkout_id TEXT NOT NULL REFERENCES checkout_sessions(id) ON DELETE CASCADE,
  product_id TEXT REFERENCES products(id) ON DELETE SET NULL,
  variant_id TEXT REFERENCES product_variants(id) ON DELETE SET NULL,
  quantity BIGINT NOT NULL,
  unit_price_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS warehouses (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, code)
);

CREATE TABLE IF NOT EXISTS stock_allocations (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT REFERENCES orders(id) ON DELETE CASCADE,
  checkout_id TEXT REFERENCES checkout_sessions(id) ON DELETE CASCADE,
  stock_item_id TEXT NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
  quantity BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stock_reservations (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  checkout_id TEXT NOT NULL REFERENCES checkout_sessions(id) ON DELETE CASCADE,
  stock_item_id TEXT NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
  quantity BIGINT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS checkout_id TEXT REFERENCES checkout_sessions(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS order_lines (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id TEXT REFERENCES products(id) ON DELETE SET NULL,
  variant_id TEXT REFERENCES product_variants(id) ON DELETE SET NULL,
  quantity BIGINT NOT NULL,
  unit_price_cents BIGINT NOT NULL,
  total_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fulfillments (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  tracking_number TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS promotion_rules (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  promotion_id TEXT NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
  rule_type TEXT NOT NULL,
  value_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vouchers (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  code TEXT NOT NULL,
  discount_type TEXT NOT NULL,
  value_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  usage_limit BIGINT,
  used_count BIGINT NOT NULL DEFAULT 0,
  starts_at TIMESTAMPTZ,
  ends_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, code)
);

CREATE TABLE IF NOT EXISTS tax_classes (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, name)
);

CREATE TABLE IF NOT EXISTS tax_rates (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  tax_class_id TEXT NOT NULL REFERENCES tax_classes(id) ON DELETE CASCADE,
  country_code TEXT NOT NULL,
  rate_basis_points BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS channels (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  default_currency TEXT NOT NULL,
  default_country TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, slug)
);

CREATE TABLE IF NOT EXISTS payments (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT REFERENCES orders(id) ON DELETE CASCADE,
  checkout_id TEXT REFERENCES checkout_sessions(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  status TEXT NOT NULL,
  amount_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  external_reference TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payment_transactions (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  payment_id TEXT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  amount_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  success BOOLEAN NOT NULL,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shipping_zones (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  countries TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shipping_methods (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  shipping_zone_id TEXT NOT NULL REFERENCES shipping_zones(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  price_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  min_order_cents BIGINT,
  max_order_cents BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS translations (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  language_code TEXT NOT NULL,
  fields JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, entity_type, entity_id, language_code)
);

CREATE TABLE IF NOT EXISTS entity_metadata (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  is_private BOOLEAN NOT NULL DEFAULT FALSE,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, entity_type, entity_id, is_private)
);

CREATE TABLE IF NOT EXISTS search_documents (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  document TSVECTOR NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS ix_search_documents_doc ON search_documents USING GIN(document);

CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  name TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  auth_token TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  app_id TEXT REFERENCES apps(id) ON DELETE CASCADE,
  event_name TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  secret TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_outbox (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  event_name TEXT NOT NULL,
  aggregate_type TEXT NOT NULL,
  aggregate_id TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  attempts BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  outbox_id TEXT NOT NULL REFERENCES event_outbox(id) ON DELETE CASCADE,
  subscription_id TEXT NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  response_status BIGINT,
  response_body TEXT,
  attempts BIGINT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ix_outbox_status_available_at ON event_outbox(status, available_at);
CREATE INDEX IF NOT EXISTS ix_webhook_deliveries_status_next_retry ON webhook_deliveries(status, next_retry_at);
