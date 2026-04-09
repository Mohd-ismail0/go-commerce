INSERT INTO permissions (id, code, description) VALUES
  ('perm_app_manage', 'app.manage', 'Manage app lifecycle and credentials')
ON CONFLICT (code) DO NOTHING;
