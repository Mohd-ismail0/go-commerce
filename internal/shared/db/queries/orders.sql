-- name: InsertOrder :one
INSERT INTO orders (
  id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, NOW(), NOW()
)
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $3, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND updated_at = $4
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at;

-- name: GetOrderByID :one
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders
WHERE id = $1 AND tenant_id = $2;

-- name: ListOrdersByTenantRegion :many
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders
WHERE tenant_id = $1
  AND ($2::text = '' OR region_id = $2)
  AND ($3::timestamptz IS NULL OR created_at < $3)
ORDER BY created_at DESC;
