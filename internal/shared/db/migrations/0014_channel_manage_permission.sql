INSERT INTO permissions (id, code, description) VALUES
  ('perm_channel_manage', 'channel.manage', 'Manage sales channels')
ON CONFLICT (code) DO NOTHING;
