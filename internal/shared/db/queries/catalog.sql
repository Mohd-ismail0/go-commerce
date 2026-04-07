-- name: UpsertProduct :one
INSERT INTO products (
  id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, NOW(), NOW()
)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  region_id = EXCLUDED.region_id,
  sku = EXCLUDED.sku,
  name = EXCLUDED.name,
  currency = EXCLUDED.currency,
  price_cents = EXCLUDED.price_cents,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at;

-- name: GetProductByID :one
SELECT id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
FROM products
WHERE id = $1 AND tenant_id = $2;

-- name: ListProductsByTenantRegion :many
SELECT id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
FROM products
WHERE tenant_id = $1
  AND ($2::text = '' OR region_id = $2)
  AND ($3::text = '' OR sku = $3)
  AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC;
