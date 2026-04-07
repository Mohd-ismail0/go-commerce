package shipping

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item ShippingMethod) ShippingMethod {
	_, _ = r.db.Exec(`
INSERT INTO shipping_methods (id, tenant_id, region_id, shipping_zone_id, name, price_cents, currency, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
name = EXCLUDED.name,
price_cents = EXCLUDED.price_cents,
currency = EXCLUDED.currency,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.ShippingZoneID, item.Name, item.PriceCents, item.Currency)
	return item
}

func (r *Repository) List(tenantID string) []ShippingMethod {
	rows, err := r.db.Query(`SELECT id, tenant_id, region_id, shipping_zone_id, name, price_cents, currency FROM shipping_methods WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return []ShippingMethod{}
	}
	defer rows.Close()
	out := []ShippingMethod{}
	for rows.Next() {
		var item ShippingMethod
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.ShippingZoneID, &item.Name, &item.PriceCents, &item.Currency); err == nil {
			out = append(out, item)
		}
	}
	return out
}
