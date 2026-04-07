package promotions

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Promotion) Promotion {
	_, _ = r.db.Exec(`
INSERT INTO promotions (id, tenant_id, region_id, name, kind, value_cents, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
kind = EXCLUDED.kind,
value_cents = EXCLUDED.value_cents,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Name, item.Kind, item.ValueCents)
	return item
}

func (r *Repository) List(tenantID string) []Promotion {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, name, kind, value_cents FROM promotions
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Promotion{}
	}
	defer rows.Close()
	out := []Promotion{}
	for rows.Next() {
		var p Promotion
		if err := rows.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Name, &p.Kind, &p.ValueCents); err == nil {
			out = append(out, p)
		}
	}
	return out
}
