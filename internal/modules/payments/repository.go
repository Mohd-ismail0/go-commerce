package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository struct {
	db *sql.DB
	q  *dbsqlc.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbsqlc.New(db)}
}

func (r *Repository) Save(ctx context.Context, item Payment) (Payment, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO payments (id, tenant_id, region_id, order_id, checkout_id, provider, status, amount_cents, currency, external_reference, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
status = EXCLUDED.status,
amount_cents = EXCLUDED.amount_cents,
external_reference = EXCLUDED.external_reference,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, nullable(item.OrderID), nullable(item.CheckoutID), item.Provider, item.Status, item.AmountCents, item.Currency, nullable(item.ExternalReference))
	if err != nil {
		return Payment{}, err
	}
	return r.GetByID(ctx, item.TenantID, item.ID)
}

func (r *Repository) List(ctx context.Context, tenantID string) ([]Payment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, tenant_id, region_id, COALESCE(order_id,''), COALESCE(checkout_id,''), provider, status, amount_cents, currency, COALESCE(external_reference,'') FROM payments WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var item Payment
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.OrderID, &item.CheckoutID, &item.Provider, &item.Status, &item.AmountCents, &item.Currency, &item.ExternalReference); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (Payment, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, COALESCE(order_id,''), COALESCE(checkout_id,''), provider, status, amount_cents, currency, COALESCE(external_reference,'')
FROM payments WHERE id = $1 AND tenant_id = $2
`, id, tenantID)
	var item Payment
	err := row.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.OrderID, &item.CheckoutID, &item.Provider, &item.Status, &item.AmountCents, &item.Currency, &item.ExternalReference)
	return item, err
}

func (r *Repository) Totals(ctx context.Context, paymentID string) (captured int64, refunded int64, err error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN event_type = $2 AND success THEN amount_cents ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN event_type = $3 AND success THEN amount_cents ELSE 0 END), 0)
FROM payment_transactions WHERE payment_id = $1
`, paymentID, EventCapture, EventRefund)
	err = row.Scan(&captured, &refunded)
	return
}

func (r *Repository) InsertTransaction(ctx context.Context, tx PaymentTransaction) (PaymentTransaction, error) {
	raw := []byte(`{}`)
	if tx.RawPayload != nil {
		b, err := json.Marshal(tx.RawPayload)
		if err != nil {
			return PaymentTransaction{}, err
		}
		raw = b
	}
	pev := nullable(tx.ProviderEventID)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO payment_transactions (id, tenant_id, region_id, payment_id, event_type, amount_cents, currency, success, raw_payload, provider_event_id, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,NOW())
`, tx.ID, tx.TenantID, tx.RegionID, tx.PaymentID, tx.EventType, tx.AmountCents, tx.Currency, tx.Success, raw, pev)
	if err != nil {
		return PaymentTransaction{}, err
	}
	return tx, nil
}

func (r *Repository) ListTransactions(ctx context.Context, tenantID, paymentID string) ([]PaymentTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, event_type, amount_cents, currency, success,
       COALESCE(provider_event_id,''),
       raw_payload
FROM payment_transactions
WHERE tenant_id = $1 AND payment_id = $2
ORDER BY created_at ASC
`, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PaymentTransaction{}
	for rows.Next() {
		var t PaymentTransaction
		var raw []byte
		if err := rows.Scan(&t.ID, &t.TenantID, &t.RegionID, &t.PaymentID, &t.EventType, &t.AmountCents, &t.Currency, &t.Success, &t.ProviderEventID, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &t.RawPayload)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) UpdatePaymentStatus(ctx context.Context, tenantID, id, status string) (Payment, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE payments SET status = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, id, tenantID, status)
	if err != nil {
		return Payment{}, err
	}
	return r.GetByID(ctx, tenantID, id)
}

func (r *Repository) FindTransactionByProviderEvent(ctx context.Context, tenantID, providerEventID string) (PaymentTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, event_type, amount_cents, currency, success,
       COALESCE(provider_event_id,''),
       raw_payload
FROM payment_transactions
WHERE tenant_id = $1 AND provider_event_id = $2
LIMIT 1
`, tenantID, providerEventID)
	var t PaymentTransaction
	var raw []byte
	err := row.Scan(&t.ID, &t.TenantID, &t.RegionID, &t.PaymentID, &t.EventType, &t.AmountCents, &t.Currency, &t.Success, &t.ProviderEventID, &raw)
	if err != nil {
		return PaymentTransaction{}, err
	}
	_ = json.Unmarshal(raw, &t.RawPayload)
	return t, nil
}

func (r *Repository) GetIdempotency(ctx context.Context, tenantID, scope, key string) (string, error) {
	return r.q.GetIdempotencyResource(ctx, tenantID, scope, key)
}

func (r *Repository) SaveIdempotency(ctx context.Context, tenantID, scope, key, resourceID string) error {
	return r.q.SaveIdempotencyResource(ctx, tenantID, scope, key, resourceID)
}

func (r *Repository) ListForRegion(ctx context.Context, tenantID, regionID string) ([]Payment, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, COALESCE(order_id,''), COALESCE(checkout_id,''), provider, status, amount_cents, currency, COALESCE(external_reference,'')
FROM payments
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var item Payment
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.OrderID, &item.CheckoutID, &item.Provider, &item.Status, &item.AmountCents, &item.Currency, &item.ExternalReference); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) SaveDispute(ctx context.Context, d Dispute) (Dispute, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO payment_disputes (id, tenant_id, region_id, payment_id, provider, provider_case_id, reason, status, amount_cents, currency, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
ON CONFLICT (tenant_id, provider, provider_case_id) DO UPDATE SET
reason = EXCLUDED.reason,
status = EXCLUDED.status,
amount_cents = EXCLUDED.amount_cents,
currency = EXCLUDED.currency,
updated_at = NOW()
`, d.ID, d.TenantID, d.RegionID, d.PaymentID, d.Provider, d.ProviderCaseID, d.Reason, d.Status, d.AmountCents, d.Currency)
	if err != nil {
		return Dispute{}, err
	}
	return d, nil
}

func (r *Repository) FindDisputeByCase(ctx context.Context, tenantID, provider, providerCaseID string) (Dispute, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, provider, provider_case_id, reason, status, amount_cents, currency
FROM payment_disputes
WHERE tenant_id = $1 AND provider = $2 AND provider_case_id = $3
LIMIT 1
`, tenantID, provider, providerCaseID)
	var d Dispute
	err := row.Scan(&d.ID, &d.TenantID, &d.RegionID, &d.PaymentID, &d.Provider, &d.ProviderCaseID, &d.Reason, &d.Status, &d.AmountCents, &d.Currency)
	return d, err
}

func (r *Repository) FindOpenReconciliationAction(ctx context.Context, tenantID, regionID, paymentID, issue string) (ReconciliationAction, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, issue, action_type, status, note
FROM payment_reconciliation_actions
WHERE tenant_id = $1 AND region_id = $2 AND payment_id = $3 AND issue = $4 AND status = 'open'
LIMIT 1
`, tenantID, regionID, paymentID, issue)
	var a ReconciliationAction
	err := row.Scan(&a.ID, &a.TenantID, &a.RegionID, &a.PaymentID, &a.Issue, &a.ActionType, &a.Status, &a.Note)
	return a, err
}

func (r *Repository) SaveReconciliationAction(ctx context.Context, a ReconciliationAction) (ReconciliationAction, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO payment_reconciliation_actions (id, tenant_id, region_id, payment_id, issue, action_type, status, note, created_at, updated_at, resolved_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW(),NULL)
`, a.ID, a.TenantID, a.RegionID, a.PaymentID, a.Issue, a.ActionType, a.Status, a.Note)
	if err != nil {
		return ReconciliationAction{}, err
	}
	return a, nil
}

func (r *Repository) ListReconciliationActions(ctx context.Context, tenantID, regionID string) ([]ReconciliationAction, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, issue, action_type, status, note
FROM payment_reconciliation_actions
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReconciliationAction{}
	for rows.Next() {
		var a ReconciliationAction
		if err := rows.Scan(&a.ID, &a.TenantID, &a.RegionID, &a.PaymentID, &a.Issue, &a.ActionType, &a.Status, &a.Note); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) ListDisputes(ctx context.Context, tenantID, regionID string) ([]Dispute, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, payment_id, provider, provider_case_id, reason, status, amount_cents, currency
FROM payment_disputes
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Dispute{}
	for rows.Next() {
		var d Dispute
		if err := rows.Scan(&d.ID, &d.TenantID, &d.RegionID, &d.PaymentID, &d.Provider, &d.ProviderCaseID, &d.Reason, &d.Status, &d.AmountCents, &d.Currency); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
