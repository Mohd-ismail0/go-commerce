package pricing

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(entry PriceBookEntry) PriceBookEntry {
	_, _ = r.db.Exec(`
INSERT INTO price_book_entries (id, tenant_id, region_id, product_id, currency, amount_cents, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
product_id = EXCLUDED.product_id,
currency = EXCLUDED.currency,
amount_cents = EXCLUDED.amount_cents,
updated_at = NOW()
`, entry.ID, entry.TenantID, entry.RegionID, entry.ProductID, entry.Currency, entry.AmountCents)
	return entry
}

func (r *Repository) List(tenantID string) []PriceBookEntry {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, product_id, currency, amount_cents FROM price_book_entries
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []PriceBookEntry{}
	}
	defer rows.Close()
	out := []PriceBookEntry{}
	for rows.Next() {
		var e PriceBookEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RegionID, &e.ProductID, &e.Currency, &e.AmountCents); err == nil {
			out = append(out, e)
		}
	}
	return out
}
