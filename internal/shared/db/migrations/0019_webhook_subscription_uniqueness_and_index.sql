CREATE UNIQUE INDEX IF NOT EXISTS ux_webhook_subscriptions_tenant_region_event_endpoint_app
ON webhook_subscriptions (tenant_id, region_id, event_name, endpoint_url, COALESCE(app_id, ''));

CREATE INDEX IF NOT EXISTS ix_webhook_subscriptions_active_lookup
ON webhook_subscriptions (tenant_id, region_id, event_name, is_active);
