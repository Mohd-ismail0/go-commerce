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
	defer func() {
		_ = rows.Close()
	}()
	out := []PriceBookEntry{}
	for rows.Next() {
		var e PriceBookEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RegionID, &e.ProductID, &e.Currency, &e.AmountCents); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func (r *Repository) SaveTaxClass(item TaxClass) TaxClass {
	_, _ = r.db.Exec(`
INSERT INTO tax_classes (id, tenant_id, region_id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Name)
	return item
}

func (r *Repository) ListTaxClasses(tenantID string) []TaxClass {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, name FROM tax_classes
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []TaxClass{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []TaxClass{}
	for rows.Next() {
		var e TaxClass
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RegionID, &e.Name); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func (r *Repository) SaveTaxRate(item TaxRate) TaxRate {
	_, _ = r.db.Exec(`
INSERT INTO tax_rates (id, tenant_id, region_id, tax_class_id, country_code, rate_basis_points, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
tax_class_id = EXCLUDED.tax_class_id,
country_code = EXCLUDED.country_code,
rate_basis_points = EXCLUDED.rate_basis_points,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.TaxClassID, item.CountryCode, item.RateBasisPoints)
	return item
}

func (r *Repository) ListTaxRates(tenantID string) []TaxRate {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, tax_class_id, country_code, rate_basis_points FROM tax_rates
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []TaxRate{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []TaxRate{}
	for rows.Next() {
		var e TaxRate
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RegionID, &e.TaxClassID, &e.CountryCode, &e.RateBasisPoints); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func (r *Repository) TaxRateBasisPoints(tenantID, regionID, taxClassID, countryCode string) int64 {
	var rate int64
	err := r.db.QueryRow(`
SELECT rate_basis_points
FROM tax_rates
WHERE tenant_id = $1
  AND region_id = $2
  AND tax_class_id = $3
  AND country_code = $4
LIMIT 1
`, tenantID, regionID, taxClassID, countryCode).Scan(&rate)
	if err != nil {
		return 0
	}
	return rate
}
