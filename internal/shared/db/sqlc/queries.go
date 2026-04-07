package sqlc

import (
	"context"
	"database/sql"
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

type ListProductsByTenantRegionParams struct {
	TenantID string
	RegionID string
	Sku      string
}

func (q *Queries) ListProductsByTenantRegion(ctx context.Context, arg ListProductsByTenantRegionParams) ([]Product, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, sku, name, currency, price_cents, created_at, updated_at
FROM products
WHERE tenant_id = $1
AND ($2::text = '' OR region_id = $2)
AND ($3::text = '' OR sku = $3)
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID, arg.Sku)
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
	ID       string
	TenantID string
	Status   string
}

func (q *Queries) UpdateOrderStatus(ctx context.Context, arg UpdateOrderStatusParams) (Order, error) {
	row := q.db.QueryRowContext(ctx, `
UPDATE orders
SET status = $3, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
`, arg.ID, arg.TenantID, arg.Status)
	var o Order
	err := row.Scan(&o.ID, &o.TenantID, &o.RegionID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

type ListOrdersByTenantRegionParams struct {
	TenantID string
	RegionID string
}

func (q *Queries) ListOrdersByTenantRegion(ctx context.Context, arg ListOrdersByTenantRegionParams) ([]Order, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders
WHERE tenant_id = $1
AND ($2::text = '' OR region_id = $2)
ORDER BY created_at DESC
`, arg.TenantID, arg.RegionID)
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
