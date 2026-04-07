package sqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Queries struct {
	db *sql.DB
}

func New(db *sql.DB) *Queries {
	return &Queries{db: db}
}

type Product struct {
	ID         string
	TenantID   string
	RegionID   string
	Sku        string
	Name       string
	Currency   string
	PriceCents int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UpsertProductParams struct {
	ID         string
	TenantID   string
	RegionID   string
	Sku        string
	Name       string
	Currency   string
	PriceCents int64
}

func (q *Queries) UpsertProduct(ctx context.Context, arg UpsertProductParams) (Product, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO products (id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
sku = EXCLUDED.sku,
name = EXCLUDED.name,
currency = EXCLUDED.currency,
price_cents = EXCLUDED.price_cents,
updated_at = NOW()
RETURNING id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.Sku, arg.Name, arg.Currency, arg.PriceCents)
	var p Product
	err := row.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Sku, &p.Name, &p.Currency, &p.PriceCents, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (q *Queries) GetProductByID(ctx context.Context, tenantID, productID string) (Product, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
FROM products WHERE id = $1 AND tenant_id = $2
`, productID, tenantID)
	var p Product
	err := row.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Sku, &p.Name, &p.Currency, &p.PriceCents, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

type ListProductsByTenantRegionParams struct {
	TenantID string
	RegionID string
	Sku      string
	Cursor   *time.Time
	Limit    int32
}

func (q *Queries) ListProductsByTenantRegion(ctx context.Context, arg ListProductsByTenantRegionParams) ([]Product, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
FROM products
WHERE tenant_id = $1
AND ($2::text = '' OR region_id = $2)
AND ($3::text = '' OR sku = $3)
AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC
LIMIT $5
`, arg.TenantID, arg.RegionID, arg.Sku, arg.Cursor, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Sku, &p.Name, &p.Currency, &p.PriceCents, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type Order struct {
	ID         string
	TenantID   string
	RegionID   string
	CustomerID string
	Status     string
	TotalCents int64
	Currency   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InsertOrderParams struct {
	ID         string
	TenantID   string
	RegionID   string
	CustomerID string
	Status     string
	TotalCents int64
	Currency   string
}

func (q *Queries) InsertOrder(ctx context.Context, arg InsertOrderParams) (Order, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO orders (id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.CustomerID, arg.Status, arg.TotalCents, arg.Currency)
	var o Order
	err := row.Scan(&o.ID, &o.TenantID, &o.RegionID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

type UpdateOrderStatusParams struct {
	ID                string
	TenantID          string
	Status            string
	ExpectedUpdatedAt time.Time
}

func (q *Queries) UpdateOrderStatus(ctx context.Context, arg UpdateOrderStatusParams) (Order, error) {
	row := q.db.QueryRowContext(ctx, `
UPDATE orders
SET status = $3, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND updated_at = $4
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
`, arg.ID, arg.TenantID, arg.Status, arg.ExpectedUpdatedAt)
	var o Order
	err := row.Scan(&o.ID, &o.TenantID, &o.RegionID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return Order{}, sql.ErrNoRows
	}
	return o, err
}

type ListOrdersByTenantRegionParams struct {
	TenantID string
	RegionID string
	Cursor   *time.Time
	Limit    int32
}

func (q *Queries) ListOrdersByTenantRegion(ctx context.Context, arg ListOrdersByTenantRegionParams) ([]Order, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders
WHERE tenant_id = $1
AND ($2::text = '' OR region_id = $2)
AND ($3::timestamptz IS NULL OR created_at < $3)
ORDER BY created_at DESC
LIMIT $4
`, arg.TenantID, arg.RegionID, arg.Cursor, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.TenantID, &o.RegionID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (q *Queries) GetIdempotencyResource(ctx context.Context, tenantID, scope, key string) (string, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT resource_id FROM idempotency_keys
WHERE tenant_id = $1 AND scope = $2 AND idempotency_key = $3
`, tenantID, scope, key)
	var resourceID string
	err := row.Scan(&resourceID)
	return resourceID, err
}

func (q *Queries) SaveIdempotencyResource(ctx context.Context, tenantID, scope, key, resourceID string) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO idempotency_keys (tenant_id, scope, idempotency_key, resource_id, created_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (tenant_id, scope, idempotency_key) DO NOTHING
`, tenantID, scope, key, resourceID)
	return err
}

func (q *Queries) InsertOrderAudit(ctx context.Context, tenantID, regionID, orderID, prevStatus, newStatus string) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO order_status_audit (tenant_id, region_id, order_id, previous_status, new_status, changed_at)
VALUES ($1, $2, $3, $4, $5, NOW())
`, tenantID, regionID, orderID, prevStatus, newStatus)
	return err
}

func (q *Queries) GetOrderByID(ctx context.Context, tenantID, orderID string) (Order, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders WHERE id = $1 AND tenant_id = $2
`, orderID, tenantID)
	var o Order
	err := row.Scan(&o.ID, &o.TenantID, &o.RegionID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

type ProductVariant struct {
	ID         string
	TenantID   string
	RegionID   string
	ProductID  string
	Sku        string
	Name       string
	PriceCents int64
	Currency   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UpsertProductVariantParams struct {
	ID         string
	TenantID   string
	RegionID   string
	ProductID  string
	Sku        string
	Name       string
	PriceCents int64
	Currency   string
}

func (q *Queries) UpsertProductVariant(ctx context.Context, arg UpsertProductVariantParams) (ProductVariant, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO product_variants (id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at)
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
RETURNING id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.ProductID, arg.Sku, arg.Name, arg.PriceCents, arg.Currency)
	var v ProductVariant
	err := row.Scan(&v.ID, &v.TenantID, &v.RegionID, &v.ProductID, &v.Sku, &v.Name, &v.PriceCents, &v.Currency, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

type ListProductVariantsByProductParams struct {
	TenantID  string
	RegionID  string
	ProductID string
}

func (q *Queries) ListProductVariantsByProduct(ctx context.Context, arg ListProductVariantsByProductParams) ([]ProductVariant, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, product_id, sku, name, price_cents, currency, created_at, updated_at
FROM product_variants
WHERE tenant_id = $1 AND region_id = $2 AND product_id = $3
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID, arg.ProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductVariant{}
	for rows.Next() {
		var v ProductVariant
		if err := rows.Scan(&v.ID, &v.TenantID, &v.RegionID, &v.ProductID, &v.Sku, &v.Name, &v.PriceCents, &v.Currency, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type SkuExistsInTenantRegionParams struct {
	TenantID  string
	RegionID  string
	Sku       string
	VariantID string
}

func (q *Queries) SkuExistsInTenantRegion(ctx context.Context, arg SkuExistsInTenantRegionParams) (bool, error) {
	row := q.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM products p
  WHERE p.tenant_id = $1 AND p.region_id = $2 AND p.sku = $3
  UNION ALL
  SELECT 1 FROM product_variants pv
  WHERE pv.tenant_id = $1 AND pv.region_id = $2 AND pv.sku = $3
    AND ($4::text = '' OR pv.id <> $4)
)
`, arg.TenantID, arg.RegionID, arg.Sku, arg.VariantID)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

type Category struct {
	ID        string
	TenantID  string
	RegionID  string
	Name      string
	Slug      string
	ParentID  sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InsertCategoryParams struct {
	ID       string
	TenantID string
	RegionID string
	Name     string
	Slug     string
	ParentID sql.NullString
}

func (q *Queries) InsertCategory(ctx context.Context, arg InsertCategoryParams) (Category, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO categories (id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.Name, arg.Slug, arg.ParentID)
	var c Category
	err := row.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Name, &c.Slug, &c.ParentID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

type ListCategoriesByTenantRegionParams struct {
	TenantID string
	RegionID string
}

func (q *Queries) ListCategoriesByTenantRegion(ctx context.Context, arg ListCategoriesByTenantRegionParams) ([]Category, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, name, slug, parent_id, created_at, updated_at
FROM categories
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Name, &c.Slug, &c.ParentID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type Collection struct {
	ID        string
	TenantID  string
	RegionID  string
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InsertCollectionParams struct {
	ID       string
	TenantID string
	RegionID string
	Name     string
	Slug     string
}

func (q *Queries) InsertCollection(ctx context.Context, arg InsertCollectionParams) (Collection, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO collections (id, tenant_id, region_id, name, slug, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
RETURNING id, tenant_id, region_id, name, slug, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.Name, arg.Slug)
	var c Collection
	err := row.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

type ListCollectionsByTenantRegionParams struct {
	TenantID string
	RegionID string
}

func (q *Queries) ListCollectionsByTenantRegion(ctx context.Context, arg ListCollectionsByTenantRegionParams) ([]Collection, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, name, slug, created_at, updated_at
FROM collections
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Collection{}
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type AssignProductToCollectionParams struct {
	TenantID     string
	RegionID     string
	CollectionID string
	ProductID    string
}

func (q *Queries) AssignProductToCollection(ctx context.Context, arg AssignProductToCollectionParams) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO collection_products (collection_id, product_id)
SELECT $3, $4
WHERE EXISTS (SELECT 1 FROM collections c WHERE c.id = $3 AND c.tenant_id = $1 AND c.region_id = $2)
AND EXISTS (SELECT 1 FROM products p WHERE p.id = $4 AND p.tenant_id = $1 AND p.region_id = $2)
ON CONFLICT (collection_id, product_id) DO NOTHING
`, arg.TenantID, arg.RegionID, arg.CollectionID, arg.ProductID)
	return err
}

type ProductMedium struct {
	ID        string
	TenantID  string
	RegionID  string
	ProductID string
	Url       string
	MediaType string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InsertProductMediaParams struct {
	ID        string
	TenantID  string
	RegionID  string
	ProductID string
	Url       string
	MediaType string
}

func (q *Queries) InsertProductMedia(ctx context.Context, arg InsertProductMediaParams) (ProductMedium, error) {
	row := q.db.QueryRowContext(ctx, `
INSERT INTO product_media (id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at
`, arg.ID, arg.TenantID, arg.RegionID, arg.ProductID, arg.Url, arg.MediaType)
	var m ProductMedium
	err := row.Scan(&m.ID, &m.TenantID, &m.RegionID, &m.ProductID, &m.Url, &m.MediaType, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

type ListProductMediaByProductParams struct {
	TenantID  string
	RegionID  string
	ProductID string
}

func (q *Queries) ListProductMediaByProduct(ctx context.Context, arg ListProductMediaByProductParams) ([]ProductMedium, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, product_id, url, media_type, created_at, updated_at
FROM product_media
WHERE tenant_id = $1 AND region_id = $2 AND product_id = $3
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID, arg.ProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductMedium{}
	for rows.Next() {
		var m ProductMedium
		if err := rows.Scan(&m.ID, &m.TenantID, &m.RegionID, &m.ProductID, &m.Url, &m.MediaType, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
