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
INSERT INTO stock_items (id, tenant_id, region_id, product_id, variant_id, warehouse_id, quantity, created_at, updated_at)
VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
product_id = EXCLUDED.product_id,
variant_id = EXCLUDED.variant_id,
warehouse_id = EXCLUDED.warehouse_id,
quantity = EXCLUDED.quantity,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.ProductID, item.VariantID, item.WarehouseID, item.Quantity)
	return item
}

func (r *Repository) List(tenantID string) []StockItem {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, COALESCE(product_id,''), COALESCE(variant_id,''), COALESCE(warehouse_id,''), quantity FROM stock_items
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []StockItem{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []StockItem{}
	for rows.Next() {
		var i StockItem
		if err := rows.Scan(&i.ID, &i.TenantID, &i.RegionID, &i.ProductID, &i.VariantID, &i.WarehouseID, &i.Quantity); err == nil {
			out = append(out, i)
		}
	}
	return out
}
