package checkout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rewrite/internal/shared/utils"
)

var ErrSessionNotFound = errors.New("checkout session not found")
var ErrSessionNotOpen = errors.New("checkout session is not open")
var ErrCheckoutEmpty = errors.New("checkout has no lines")
var ErrInsufficientStock = errors.New("insufficient stock for checkout line")
var ErrVoucherUnavailable = errors.New("voucher is unavailable")

type Repository interface {
	CreateSession(ctx context.Context, in Session) (Session, error)
	UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session) (Session, error)
	UpsertLine(ctx context.Context, tenantID, regionID string, line Line) (Line, error)
	Recalculate(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error)
	UpdatePricing(ctx context.Context, tenantID, regionID, checkoutID string, taxCents, totalCents int64) (Session, error)
	Complete(ctx context.Context, tenantID, regionID, checkoutID, orderID string) (OrderCreatedPayload, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn}
}

func (r *PostgresRepository) CreateSession(ctx context.Context, in Session) (Session, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO checkout_sessions (id, tenant_id, region_id, customer_id, status, currency, voucher_code, promotion_id, tax_class_id, country_code, subtotal_cents, shipping_cents, tax_cents, total_cents, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),0,0,0,0,NOW(),NOW())
RETURNING id, tenant_id, region_id, customer_id, status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
`, in.ID, in.TenantID, in.RegionID, in.CustomerID, in.Status, in.Currency, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode)
	return scanSession(row)
}

func (r *PostgresRepository) UpsertLine(ctx context.Context, tenantID, regionID string, line Line) (Line, error) {
	if !r.sessionExists(ctx, tenantID, regionID, line.CheckoutID) {
		return Line{}, ErrSessionNotFound
	}
	if line.ID != "" {
		row := r.db.QueryRowContext(ctx, `
UPDATE checkout_lines
SET product_id = NULLIF($6,''), variant_id = NULLIF($7,''), quantity = $4, unit_price_cents = $5, currency = $8, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND checkout_id = $9
RETURNING id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
`, line.ID, tenantID, regionID, line.Quantity, line.UnitPriceCents, line.ProductID, line.VariantID, line.Currency, line.CheckoutID)
		out, err := scanLine(row)
		if err == nil {
			return out, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Line{}, err
		}
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO checkout_lines (id, tenant_id, region_id, checkout_id, product_id, variant_id, quantity, unit_price_cents, currency, created_at, updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,NOW(),NOW())
RETURNING id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
`, line.ID, tenantID, regionID, line.CheckoutID, line.ProductID, line.VariantID, line.Quantity, line.UnitPriceCents, line.Currency)
	return scanLine(row)
}

func (r *PostgresRepository) UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session) (Session, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE checkout_sessions
SET voucher_code = NULLIF($4,''),
    promotion_id = NULLIF($5,''),
    tax_class_id = NULLIF($6,''),
    country_code = NULLIF($7,''),
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode)
	if err != nil {
		return Session{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Session{}, ErrSessionNotFound
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) Recalculate(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	if !r.sessionExists(ctx, tenantID, regionID, checkoutID) {
		return Session{}, ErrSessionNotFound
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE checkout_sessions
SET subtotal_cents = COALESCE((SELECT SUM(quantity * unit_price_cents) FROM checkout_lines WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3), 0),
    total_cents = COALESCE((SELECT SUM(quantity * unit_price_cents) FROM checkout_lines WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3), 0)
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	if err != nil {
		return Session{}, err
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) UpdatePricing(ctx context.Context, tenantID, regionID, checkoutID string, taxCents, totalCents int64) (Session, error) {
	_, err := r.db.ExecContext(ctx, `
UPDATE checkout_sessions
SET tax_cents = $4, total_cents = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID, taxCents, totalCents)
	if err != nil {
		return Session{}, err
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) Complete(ctx context.Context, tenantID, regionID, checkoutID, orderID string) (OrderCreatedPayload, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return OrderCreatedPayload{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var session Session
	row := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, checkoutID, tenantID, regionID)
	if scanErr := scanSessionInto(row, &session); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return OrderCreatedPayload{}, ErrSessionNotFound
		}
		return OrderCreatedPayload{}, scanErr
	}
	if session.Status != "open" {
		return OrderCreatedPayload{}, ErrSessionNotOpen
	}

	lines, err := tx.QueryContext(ctx, `
SELECT id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
FROM checkout_lines
WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	if err != nil {
		return OrderCreatedPayload{}, err
	}
	defer func() {
		_ = lines.Close()
	}()
	collected := make([]Line, 0)
	for lines.Next() {
		var l Line
		if err = lines.Scan(&l.ID, &l.CheckoutID, &l.ProductID, &l.VariantID, &l.Quantity, &l.UnitPriceCents, &l.Currency); err != nil {
			return OrderCreatedPayload{}, err
		}
		collected = append(collected, l)
	}
	if len(collected) == 0 {
		return OrderCreatedPayload{}, ErrCheckoutEmpty
	}
	if err = lines.Err(); err != nil {
		return OrderCreatedPayload{}, err
	}

	// Wave 1 invariant: reserve stock atomically during checkout completion.
	requiredByStockItem := map[string]int64{}
	for _, l := range collected {
		stockItemID, resolveErr := resolveStockItemID(ctx, tx, tenantID, regionID, l)
		if resolveErr != nil {
			return OrderCreatedPayload{}, resolveErr
		}
		requiredByStockItem[stockItemID] += l.Quantity
	}
	for stockItemID, required := range requiredByStockItem {
		var available int64
		row := tx.QueryRowContext(ctx, `
SELECT quantity
FROM stock_items
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
FOR UPDATE
`, tenantID, regionID, stockItemID)
		if scanErr := row.Scan(&available); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return OrderCreatedPayload{}, ErrInsufficientStock
			}
			return OrderCreatedPayload{}, scanErr
		}
		if available < required {
			return OrderCreatedPayload{}, ErrInsufficientStock
		}
		if _, err = tx.ExecContext(ctx, `
UPDATE stock_items
SET quantity = quantity - $4, updated_at = NOW()
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, stockItemID, required); err != nil {
			return OrderCreatedPayload{}, err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO stock_allocations (id, tenant_id, region_id, order_id, checkout_id, stock_item_id, quantity, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
`, utils.NewID("alc"), tenantID, regionID, orderID, checkoutID, stockItemID, required); err != nil {
			return OrderCreatedPayload{}, err
		}
	}

	if session.VoucherCode != "" {
		consumeVoucher := tx.QueryRowContext(ctx, `
UPDATE vouchers
SET used_count = used_count + 1, updated_at = NOW()
WHERE id = (
  SELECT id
  FROM vouchers
  WHERE tenant_id = $1
    AND region_id = $2
    AND code = $3
    AND currency = $4
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
    AND (usage_limit IS NULL OR used_count < usage_limit)
  LIMIT 1
)
RETURNING id
`, tenantID, regionID, session.VoucherCode, session.Currency)
		var voucherID string
		if scanErr := consumeVoucher.Scan(&voucherID); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return OrderCreatedPayload{}, ErrVoucherUnavailable
			}
			return OrderCreatedPayload{}, scanErr
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO orders (id, tenant_id, region_id, customer_id, status, total_cents, currency, checkout_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,'created',$5,$6,$7,NOW(),NOW())
`, orderID, tenantID, regionID, session.CustomerID, session.TotalCents, session.Currency, session.ID); err != nil {
		return OrderCreatedPayload{}, err
	}
	for _, l := range collected {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO order_lines (id, tenant_id, region_id, order_id, product_id, variant_id, quantity, unit_price_cents, total_cents, currency, created_at, updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,NOW(),NOW())
`, "ol_"+l.ID, tenantID, regionID, orderID, l.ProductID, l.VariantID, l.Quantity, l.UnitPriceCents, l.Quantity*l.UnitPriceCents, l.Currency); err != nil {
			return OrderCreatedPayload{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE checkout_sessions SET status = 'completed', updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID); err != nil {
		return OrderCreatedPayload{}, err
	}
	if err = tx.Commit(); err != nil {
		return OrderCreatedPayload{}, err
	}
	return OrderCreatedPayload{
		ID:         orderID,
		TenantID:   tenantID,
		RegionID:   regionID,
		CustomerID: session.CustomerID,
		Status:     "created",
		TotalCents: session.TotalCents,
		Currency:   session.Currency,
		CheckoutID: session.ID,
	}, nil
}

func resolveStockItemID(ctx context.Context, tx *sql.Tx, tenantID, regionID string, line Line) (string, error) {
	var stockItemID string
	if line.VariantID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT s.id
FROM stock_items s
LEFT JOIN warehouses w ON w.id = s.warehouse_id AND w.tenant_id = s.tenant_id AND w.region_id = s.region_id
WHERE s.tenant_id = $1 AND s.region_id = $2 AND s.variant_id = $3
  AND (s.warehouse_id IS NULL OR COALESCE(w.is_active, FALSE) = TRUE)
ORDER BY s.quantity DESC, s.updated_at ASC
LIMIT 1
`, tenantID, regionID, line.VariantID)
		if err := row.Scan(&stockItemID); err == nil {
			return stockItemID, nil
		}
	}
	if line.ProductID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT s.id
FROM stock_items s
LEFT JOIN warehouses w ON w.id = s.warehouse_id AND w.tenant_id = s.tenant_id AND w.region_id = s.region_id
WHERE s.tenant_id = $1 AND s.region_id = $2 AND s.product_id = $3 AND (s.variant_id IS NULL OR s.variant_id = '')
  AND (s.warehouse_id IS NULL OR COALESCE(w.is_active, FALSE) = TRUE)
ORDER BY s.quantity DESC, s.updated_at ASC
LIMIT 1
`, tenantID, regionID, line.ProductID)
		if err := row.Scan(&stockItemID); err == nil {
			return stockItemID, nil
		}
	}
	return "", ErrInsufficientStock
}

func (r *PostgresRepository) sessionExists(ctx context.Context, tenantID, regionID, checkoutID string) bool {
	var found int
	err := r.db.QueryRowContext(ctx, `
SELECT 1 FROM checkout_sessions WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID).Scan(&found)
	return err == nil
}

func (r *PostgresRepository) getSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	return session, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (Session, error) {
	var out Session
	err := scanSessionInto(row, &out)
	return out, err
}

func scanSessionInto(row scanner, out *Session) error {
	var updatedAt time.Time
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.RegionID,
		&out.CustomerID,
		&out.Status,
		&out.Currency,
		&out.VoucherCode,
		&out.PromotionID,
		&out.TaxClassID,
		&out.CountryCode,
		&out.SubtotalCents,
		&out.ShippingCents,
		&out.TaxCents,
		&out.TotalCents,
		&updatedAt,
	); err != nil {
		return err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return nil
}

func scanLine(row scanner) (Line, error) {
	var out Line
	err := row.Scan(
		&out.ID,
		&out.CheckoutID,
		&out.ProductID,
		&out.VariantID,
		&out.Quantity,
		&out.UnitPriceCents,
		&out.Currency,
	)
	if err != nil {
		return Line{}, fmt.Errorf("scan checkout line: %w", err)
	}
	return out, nil
}
