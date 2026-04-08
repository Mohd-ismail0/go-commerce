ALTER TABLE payment_transactions ADD COLUMN IF NOT EXISTS provider_event_id TEXT;

ALTER TABLE shipping_zones ALTER COLUMN countries TYPE JSONB USING to_jsonb(countries);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_tx_tenant_provider_event
ON payment_transactions (tenant_id, provider_event_id)
WHERE provider_event_id IS NOT NULL AND btrim(provider_event_id) <> '';

ALTER TABLE shipping_methods ADD COLUMN IF NOT EXISTS channel_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE shipping_methods ADD COLUMN IF NOT EXISTS postal_prefixes JSONB NOT NULL DEFAULT '[]'::jsonb;

INSERT INTO permissions (id, code, description) VALUES
  ('perm_pay_manage', 'payments.manage', 'Create and manage payments'),
  ('perm_ship_manage', 'shipping.manage', 'Manage shipping zones and methods'),
  ('perm_meta_manage', 'metadata.manage', 'Manage entity metadata'),
  ('perm_identity_users', 'identity.users.manage', 'Manage users')
ON CONFLICT (code) DO NOTHING;
