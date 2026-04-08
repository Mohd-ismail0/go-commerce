INSERT INTO permissions (code, description)
VALUES ('webhook.manage', 'Manage webhook subscriptions')
ON CONFLICT (code) DO NOTHING;
