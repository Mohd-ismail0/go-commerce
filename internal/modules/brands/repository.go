package brands

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(brand Brand) Brand {
	_, _ = r.db.Exec(`
INSERT INTO brands (id, tenant_id, region_id, name, default_locale, default_currency, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
default_locale = EXCLUDED.default_locale,
default_currency = EXCLUDED.default_currency,
updated_at = NOW()
`, brand.ID, brand.TenantID, brand.RegionID, brand.Name, brand.DefaultLocale, brand.DefaultCurrency)
	return brand
}

func (r *Repository) List(tenantID string) []Brand {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, name, default_locale, default_currency FROM brands
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Brand{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Brand{}
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.TenantID, &b.RegionID, &b.Name, &b.DefaultLocale, &b.DefaultCurrency); err == nil {
			out = append(out, b)
		}
	}
	return out
}
