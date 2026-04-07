package inventory

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item StockItem) StockItem {
	_, _ = r.db.Exec(`
INSERT INTO stock_items (id, tenant_id, region_id, product_id, quantity, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
product_id = EXCLUDED.product_id,
quantity = EXCLUDED.quantity,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.ProductID, item.Quantity)
	return item
}

func (r *Repository) List(tenantID string) []StockItem {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, product_id, quantity FROM stock_items
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []StockItem{}
	}
	defer rows.Close()
	out := []StockItem{}
	for rows.Next() {
		var i StockItem
		if err := rows.Scan(&i.ID, &i.TenantID, &i.RegionID, &i.ProductID, &i.Quantity); err == nil {
			out = append(out, i)
		}
	}
	return out
}
