package inventory

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

func (r *Repository) ListWarehouses(ctx context.Context, tenantID, regionID string) ([]Warehouse, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, name, code, is_active
FROM warehouses WHERE tenant_id = $1 AND region_id = $2 ORDER BY name ASC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Warehouse
	for rows.Next() {
		var w Warehouse
		if err := rows.Scan(&w.ID, &w.TenantID, &w.RegionID, &w.Name, &w.Code, &w.IsActive); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *Repository) SaveWarehouse(ctx context.Context, w Warehouse) (Warehouse, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO warehouses (id, tenant_id, region_id, name, code, is_active, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
name = EXCLUDED.name,
code = EXCLUDED.code,
is_active = EXCLUDED.is_active,
updated_at = NOW()
`, w.ID, w.TenantID, w.RegionID, w.Name, w.Code, w.IsActive)
	if err != nil {
		return Warehouse{}, err
	}
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, name, code, is_active FROM warehouses WHERE tenant_id = $1 AND id = $2
`, w.TenantID, w.ID)
	var out Warehouse
	if err := row.Scan(&out.ID, &out.TenantID, &out.RegionID, &out.Name, &out.Code, &out.IsActive); err != nil {
		return Warehouse{}, err
	}
	return out, nil
}
