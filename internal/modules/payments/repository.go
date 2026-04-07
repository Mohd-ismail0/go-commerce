package payments

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Payment) Payment {
	_, _ = r.db.Exec(`
INSERT INTO payments (id, tenant_id, region_id, order_id, checkout_id, provider, status, amount_cents, currency, external_reference, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
status = EXCLUDED.status,
amount_cents = EXCLUDED.amount_cents,
external_reference = EXCLUDED.external_reference,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, nullable(item.OrderID), nullable(item.CheckoutID), item.Provider, item.Status, item.AmountCents, item.Currency, nullable(item.ExternalReference))
	return item
}

func (r *Repository) List(tenantID string) []Payment {
	rows, err := r.db.Query(`SELECT id, tenant_id, region_id, COALESCE(order_id,''), COALESCE(checkout_id,''), provider, status, amount_cents, currency, COALESCE(external_reference,'') FROM payments WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return []Payment{}
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var item Payment
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.OrderID, &item.CheckoutID, &item.Provider, &item.Status, &item.AmountCents, &item.Currency, &item.ExternalReference); err == nil {
			out = append(out, item)
		}
	}
	return out
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
