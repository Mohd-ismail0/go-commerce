CREATE TABLE IF NOT EXISTS gift_cards (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  code TEXT NOT NULL,
  balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0),
  currency TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, code)
);

CREATE INDEX IF NOT EXISTS ix_gift_cards_tenant_region_active
ON gift_cards (tenant_id, region_id, is_active, updated_at DESC);

CREATE TABLE IF NOT EXISTS order_invoices (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  region_id TEXT NOT NULL,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  invoice_number TEXT NOT NULL,
  status TEXT NOT NULL,
  total_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, region_id, invoice_number),
  CONSTRAINT chk_order_invoices_status CHECK (status IN ('draft', 'issued', 'void'))
);

CREATE INDEX IF NOT EXISTS ix_order_invoices_order
ON order_invoices (tenant_id, region_id, order_id, created_at DESC);

ALTER TABLE checkout_sessions
  ADD COLUMN IF NOT EXISTS gift_card_id TEXT REFERENCES gift_cards(id) ON DELETE SET NULL;

ALTER TABLE checkout_sessions
  ADD COLUMN IF NOT EXISTS gift_card_applied_cents BIGINT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS ux_checkout_open_unique_gift_card
ON checkout_sessions (gift_card_id)
WHERE gift_card_id IS NOT NULL AND status = 'open';

INSERT INTO permissions (id, code, description) VALUES
  ('perm_gift_cards_manage', 'gift_cards.manage', 'Create and list gift cards'),
  ('perm_invoices_manage', 'invoices.manage', 'Create and list order invoices')
ON CONFLICT (code) DO NOTHING;
