-- name: InsertProductType :one
INSERT INTO product_types (id, tenant_id, region_id, name, slug, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, created_at, updated_at;

-- name: ListProductTypesByTenantRegion :many
SELECT id, tenant_id, region_id, name, slug, created_at, updated_at
FROM product_types
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC;

-- name: GetProductTypeByID :one
SELECT id, tenant_id, region_id, name, slug, created_at, updated_at
FROM product_types
WHERE id = $1 AND tenant_id = $2 AND region_id = $3;

-- name: InsertCatalogAttribute :one
INSERT INTO catalog_attributes (id, tenant_id, region_id, name, slug, input_type, unit, allowed_values, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, input_type, unit, allowed_values, created_at, updated_at;

-- name: ListCatalogAttributesByTenantRegion :many
SELECT id, tenant_id, region_id, name, slug, input_type, unit, allowed_values, created_at, updated_at
FROM catalog_attributes
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC;

-- name: GetCatalogAttributeByID :one
SELECT id, tenant_id, region_id, name, slug, input_type, unit, allowed_values, created_at, updated_at
FROM catalog_attributes
WHERE id = $1 AND tenant_id = $2 AND region_id = $3;

-- name: LinkAttributeToProductType :exec
INSERT INTO product_type_attributes (product_type_id, attribute_id, sort_order, variant_only)
VALUES ($1, $2, $3, $4)
ON CONFLICT (product_type_id, attribute_id) DO UPDATE SET
  sort_order = EXCLUDED.sort_order,
  variant_only = EXCLUDED.variant_only;

-- name: UnlinkAttributeFromProductType :exec
DELETE FROM product_type_attributes
WHERE product_type_id = $1 AND attribute_id = $2;

-- name: ListProductTypeAttributeRows :many
SELECT
  pta.attribute_id,
  pta.sort_order,
  pta.variant_only,
  ca.name AS attribute_name,
  ca.slug AS attribute_slug,
  ca.input_type,
  ca.unit,
  ca.allowed_values
FROM product_type_attributes pta
JOIN catalog_attributes ca ON ca.id = pta.attribute_id
JOIN product_types pt ON pt.id = pta.product_type_id
WHERE pta.product_type_id = $1 AND pt.tenant_id = $2 AND pt.region_id = $3
ORDER BY pta.sort_order ASC, pta.attribute_id ASC;

-- name: GetProductTypeAttributeAssignment :one
SELECT variant_only
FROM product_type_attributes pta
JOIN product_types pt ON pt.id = pta.product_type_id
WHERE pta.product_type_id = $1 AND pta.attribute_id = $2 AND pt.tenant_id = $3 AND pt.region_id = $4;

-- name: GetProductRegionAndType :one
SELECT region_id, product_type_id FROM products WHERE id = $1 AND tenant_id = $2;

-- name: GetVariantProductRegion :one
SELECT product_id, region_id FROM product_variants WHERE id = $1 AND tenant_id = $2;

-- name: UpsertProductAttributeValue :exec
INSERT INTO product_attribute_values (product_id, attribute_id, value_text, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
ON CONFLICT (product_id, attribute_id) DO UPDATE SET
  value_text = EXCLUDED.value_text,
  updated_at = NOW();

-- name: UpsertVariantAttributeValue :exec
INSERT INTO variant_attribute_values (variant_id, attribute_id, value_text, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
ON CONFLICT (variant_id, attribute_id) DO UPDATE SET
  value_text = EXCLUDED.value_text,
  updated_at = NOW();

-- name: ListProductAttributeValues :many
SELECT attribute_id, value_text
FROM product_attribute_values
WHERE product_id = $1
ORDER BY attribute_id;

-- name: ListVariantAttributeValues :many
SELECT attribute_id, value_text
FROM variant_attribute_values
WHERE variant_id = $1
ORDER BY attribute_id;

-- name: ListProductAttributeValuesForProducts :many
SELECT product_id, attribute_id, value_text
FROM product_attribute_values
WHERE product_id = ANY($1::text[])
ORDER BY product_id, attribute_id;

-- name: CleanupProductAttributeValuesForProduct :exec
DELETE FROM product_attribute_values pav
USING products p
WHERE pav.product_id = p.id
  AND p.id = $1
  AND p.tenant_id = $2
  AND (
    p.product_type_id IS NULL
    OR NOT EXISTS (
      SELECT 1 FROM product_type_attributes pta
      WHERE pta.product_type_id = p.product_type_id
        AND pta.attribute_id = pav.attribute_id
        AND pta.variant_only = false
    )
  );

-- name: CleanupVariantAttributeValuesForProduct :exec
DELETE FROM variant_attribute_values vav
USING product_variants v, products p
WHERE vav.variant_id = v.id
  AND v.product_id = p.id
  AND p.id = $1
  AND p.tenant_id = $2
  AND (
    p.product_type_id IS NULL
    OR NOT EXISTS (
      SELECT 1 FROM product_type_attributes pta
      WHERE pta.product_type_id = p.product_type_id
        AND pta.attribute_id = vav.attribute_id
        AND pta.variant_only = true
    )
  );

-- name: DeleteProductAttributeValuesForUnlinkedAttribute :exec
DELETE FROM product_attribute_values pav
USING products p
WHERE pav.product_id = p.id
  AND p.product_type_id = sqlc.arg(product_type_id)::text
  AND pav.attribute_id = sqlc.arg(attribute_id);

-- name: DeleteVariantAttributeValuesForUnlinkedAttribute :exec
DELETE FROM variant_attribute_values vav
USING product_variants v, products p
WHERE vav.variant_id = v.id
  AND v.product_id = p.id
  AND p.product_type_id = sqlc.arg(product_type_id)::text
  AND vav.attribute_id = sqlc.arg(attribute_id);
