package customers

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(customer Customer) Customer {
	_, _ = r.db.Exec(`
INSERT INTO customers (id, tenant_id, region_id, email, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
email = EXCLUDED.email,
name = EXCLUDED.name,
updated_at = NOW()
`, customer.ID, customer.TenantID, customer.RegionID, customer.Email, customer.Name)
	return customer
}

func (r *Repository) List(tenantID string) []Customer {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, email, name FROM customers
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Customer{}
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Email, &c.Name); err == nil {
			out = append(out, c)
		}
	}
	return out
}
