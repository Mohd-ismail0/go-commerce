package invoice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Repository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetByID(ctx context.Context, tenantID, regionID, id string) (Invoice, error) {
	row := r.DB.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, order_id, invoice_number, status, total_cents, currency, metadata, created_at, updated_at
FROM order_invoices
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, id, tenantID, regionID)
	return scanInvoice(row)
}

func (r *Repository) ListByOrder(ctx context.Context, tenantID, regionID, orderID string) ([]Invoice, error) {
	rows, err := r.DB.QueryContext(ctx, `
SELECT id, tenant_id, region_id, order_id, invoice_number, status, total_cents, currency, metadata, created_at, updated_at
FROM order_invoices
WHERE tenant_id = $1 AND region_id = $2 AND order_id = $3
ORDER BY created_at DESC
`, tenantID, regionID, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Invoice
	for rows.Next() {
		inv, err := scanInvoiceRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func scanInvoice(row *sql.Row) (Invoice, error) {
	var inv Invoice
	var meta []byte
	var ca, ua sql.NullTime
	if err := row.Scan(&inv.ID, &inv.TenantID, &inv.RegionID, &inv.OrderID, &inv.InvoiceNumber, &inv.Status, &inv.TotalCents, &inv.Currency, &meta, &ca, &ua); err != nil {
		return Invoice{}, err
	}
	if len(meta) > 0 {
		inv.Metadata = meta
	}
	if ca.Valid {
		inv.CreatedAt = ca.Time.UTC().Format(time.RFC3339Nano)
	}
	if ua.Valid {
		inv.UpdatedAt = ua.Time.UTC().Format(time.RFC3339Nano)
	}
	return inv, nil
}

func scanInvoiceRows(rows *sql.Rows) (Invoice, error) {
	var inv Invoice
	var meta []byte
	var ca, ua sql.NullTime
	if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.RegionID, &inv.OrderID, &inv.InvoiceNumber, &inv.Status, &inv.TotalCents, &inv.Currency, &meta, &ca, &ua); err != nil {
		return Invoice{}, err
	}
	if len(meta) > 0 {
		inv.Metadata = meta
	}
	if ca.Valid {
		inv.CreatedAt = ca.Time.UTC().Format(time.RFC3339Nano)
	}
	if ua.Valid {
		inv.UpdatedAt = ua.Time.UTC().Format(time.RFC3339Nano)
	}
	return inv, nil
}

var ErrOrderNotFound = errors.New("order not found")

func (r *Repository) InsertInTx(ctx context.Context, tx *sql.Tx, inv Invoice) (Invoice, error) {
	meta := inv.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	if !json.Valid(meta) {
		meta = []byte("{}")
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO order_invoices (id, tenant_id, region_id, order_id, invoice_number, status, total_cents, currency, metadata, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,NOW(),NOW())
RETURNING id, tenant_id, region_id, order_id, invoice_number, status, total_cents, currency, metadata, created_at, updated_at
`, inv.ID, inv.TenantID, inv.RegionID, inv.OrderID, inv.InvoiceNumber, inv.Status, inv.TotalCents, inv.Currency, string(meta))
	return scanInvoice(row)
}

func (r *Repository) LockOrderTotals(ctx context.Context, tx *sql.Tx, tenantID, regionID, orderID string) (totalCents int64, currency string, err error) {
	err = tx.QueryRowContext(ctx, `
SELECT total_cents, currency
FROM orders
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, orderID, tenantID, regionID).Scan(&totalCents, &currency)
	return totalCents, currency, err
}
