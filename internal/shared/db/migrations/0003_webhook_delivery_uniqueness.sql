CREATE UNIQUE INDEX IF NOT EXISTS ux_webhook_deliveries_outbox_subscription
ON webhook_deliveries(outbox_id, subscription_id);
