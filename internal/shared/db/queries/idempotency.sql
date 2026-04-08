-- name: GetIdempotencyResource :one
SELECT resource_id
FROM idempotency_keys
WHERE tenant_id = $1
  AND scope = $2
  AND idempotency_key = $3;

-- name: SaveIdempotencyResource :exec
INSERT INTO idempotency_keys (tenant_id, scope, idempotency_key, resource_id, created_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (tenant_id, scope, idempotency_key) DO NOTHING;

-- name: InsertOrderAudit :exec
INSERT INTO order_status_audit (tenant_id, region_id, order_id, previous_status, new_status, changed_at)
VALUES ($1, $2, $3, $4, $5, NOW());
