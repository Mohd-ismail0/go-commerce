-- name: UpsertProduct :one
INSERT INTO products (
  id, tenant_id, region_id, sku, name, slug, description, seo_title, seo_description, metadata, external_reference, currency, price_cents, created_at, updated_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW()
)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  region_id = EXCLUDED.region_id,
  sku = EXCLUDED.sku,
  name = EXCLUDED.name,
  slug = EXCLUDED.slug,
  description = EXCLUDED.description,
  seo_title = EXCLUDED.seo_title,
  seo_description = EXCLUDED.seo_description,
  metadata = EXCLUDED.metadata,
  external_reference = EXCLUDED.external_reference,
  currency = EXCLUDED.currency,
  price_cents = EXCLUDED.price_cents,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, sku, name, slug, description, seo_title, seo_description, metadata, external_reference, currency, price_cents, created_at, updated_at;

-- name: GetProductByID :one
SELECT id, tenant_id, region_id, sku, name, slug, description, seo_title, seo_description, metadata, external_reference, currency, price_cents, created_at, updated_at
FROM products
WHERE id = $1 AND tenant_id = $2;

-- name: ListProductsByTenantRegion :many
SELECT id, tenant_id, region_id, sku, name, slug, description, seo_title, seo_description, metadata, external_reference, currency, price_cents, created_at, updated_at
FROM products
WHERE tenant_id = $1
  AND ($2::text = '' OR region_id = $2)
  AND ($3::text = '' OR sku = $3)
  AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC
LIMIT $5;

-- name: UpsertProductVariant :one
INSERT INTO product_variants (
  id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  region_id = EXCLUDED.region_id,
  product_id = EXCLUDED.product_id,
  sku = EXCLUDED.sku,
  name = EXCLUDED.name,
  price_cents = EXCLUDED.price_cents,
  currency = EXCLUDED.currency,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at;

-- name: ListProductVariantsByProduct :many
SELECT id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at
FROM product_variants
WHERE tenant_id = $1
  AND region_id = $2
  AND product_id = $3
ORDER BY created_at DESC;

-- name: SkuExistsInTenantRegion :one
SELECT EXISTS (
  SELECT 1 FROM products p
  WHERE p.tenant_id = $1 AND p.region_id = $2 AND p.sku = $3
  UNION ALL
  SELECT 1 FROM product_variants pv
  WHERE pv.tenant_id = $1 AND pv.region_id = $2 AND pv.sku = $3
    AND ($4::text = '' OR pv.id <> $4)
);

-- name: InsertCategory :one
INSERT INTO categories (id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at;

-- name: ListCategoriesByTenantRegion :many
SELECT id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at
FROM categories
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC;

-- name: InsertCollection :one
INSERT INTO collections (id, tenant_id, region_id, name, slug, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, created_at, updated_at;

-- name: ListCollectionsByTenantRegion :many
SELECT id, tenant_id, region_id, name, slug, created_at, updated_at
FROM collections
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC;

-- name: AssignProductToCollection :exec
INSERT INTO collection_products (collection_id, product_id)
SELECT $3, $4
WHERE EXISTS (
  SELECT 1 FROM collections c
  WHERE c.id = $3 AND c.tenant_id = $1 AND c.region_id = $2
)
AND EXISTS (
  SELECT 1 FROM products p
  WHERE p.id = $4 AND p.tenant_id = $1 AND p.region_id = $2
)
ON CONFLICT (collection_id, product_id) DO NOTHING;

-- name: InsertProductMedia :one
INSERT INTO product_media (id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at;

-- name: ListProductMediaByProduct :many
SELECT id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at
FROM product_media
WHERE tenant_id = $1 AND region_id = $2 AND product_id = $3
ORDER BY created_at DESC;
