package regions

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Region) Region {
	_, _ = r.db.Exec(`
INSERT INTO regions (id, tenant_id, region_id, name, currency, locale_code, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
currency = EXCLUDED.currency,
locale_code = EXCLUDED.locale_code,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Name, item.Currency, item.LocaleCode)
	return item
}

func (r *Repository) List(tenantID string) []Region {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, name, currency, locale_code FROM regions
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Region{}
	}
	defer rows.Close()
	out := []Region{}
	for rows.Next() {
		var i Region
		if err := rows.Scan(&i.ID, &i.TenantID, &i.RegionID, &i.Name, &i.Currency, &i.LocaleCode); err == nil {
			out = append(out, i)
		}
	}
	return out
}
