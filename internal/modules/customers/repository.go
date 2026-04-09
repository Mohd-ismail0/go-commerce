package customers

import (
	"context"
	"database/sql"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, customer Customer) (Customer, error) {
	var out Customer
	err := r.db.QueryRowContext(ctx, `
INSERT INTO customers (id, tenant_id, region_id, email, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
email = EXCLUDED.email,
name = EXCLUDED.name,
updated_at = NOW()
RETURNING id, tenant_id, region_id, email, name
`, customer.ID, customer.TenantID, customer.RegionID, customer.Email, customer.Name).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.Email, &out.Name,
	)
	if err != nil {
		return Customer{}, err
	}
	return out, nil
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string) ([]Customer, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, email, name FROM customers
WHERE tenant_id = $1 AND region_id = $2 ORDER BY updated_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Customer{}
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Email, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) EmailTaken(ctx context.Context, tenantID, regionID, email, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM customers
	WHERE tenant_id = $1
	  AND region_id = $2
	  AND LOWER(email) = LOWER($3)
	  AND ($4::text = '' OR id <> $4)
)
`, tenantID, regionID, email, excludeID).Scan(&exists)
	return exists, err
}
