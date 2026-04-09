CREATE UNIQUE INDEX IF NOT EXISTS ux_apps_tenant_region_name_ci
ON apps (tenant_id, region_id, lower(name));
