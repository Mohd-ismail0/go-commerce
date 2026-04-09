package checkout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"rewrite/internal/shared/utils"
)

var ErrSessionNotFound = errors.New("checkout session not found")
var ErrSessionNotOpen = errors.New("checkout session is not open")
var ErrCheckoutEmpty = errors.New("checkout has no lines")
var ErrInsufficientStock = errors.New("insufficient stock for checkout line")
var ErrVoucherUnavailable = errors.New("voucher is unavailable")
var ErrChannelListingMismatch = errors.New("checkout line no longer matches channel listing")

type Repository interface {
	CreateSession(ctx context.Context, in Session) (Session, error)
	UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session) (Session, error)
	UpsertLine(ctx context.Context, tenantID, regionID string, line Line) (Line, error)
	GetSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error)
	ListLines(ctx context.Context, tenantID, regionID, checkoutID string) ([]Line, error)
	ChannelIsActive(ctx context.Context, tenantID, regionID, channelID string) (bool, error)
	GetProductChannelListing(ctx context.Context, tenantID, regionID, channelID, productID string) (bool, bool, error)
	GetVariantChannelListing(ctx context.Context, tenantID, regionID, channelID, variantID string) (int64, string, bool, bool, error)
	GetVariantProductID(ctx context.Context, tenantID, regionID, variantID string) (string, bool, error)
	ResolveShippingMethodPrice(ctx context.Context, tenantID, regionID, shippingMethodID, countryCode, channelID, postalCode, currency string, subtotalCents int64) (int64, bool, error)
	UpdateShippingCents(ctx context.Context, tenantID, regionID, checkoutID string, shippingCents int64) (Session, error)
	HasAuthorizedPaymentCoverage(ctx context.Context, tenantID, regionID, checkoutID string, requiredTotalCents int64) (bool, error)
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
INSERT INTO checkout_sessions (id, tenant_id, region_id, customer_id, channel_id, shipping_method_id, shipping_address_country, shipping_address_postal_code, billing_address_country, billing_address_postal_code, status, currency, voucher_code, promotion_id, tax_class_id, country_code, subtotal_cents, shipping_cents, tax_cents, total_cents, created_at, updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),0,0,0,0,NOW(),NOW())
RETURNING id, tenant_id, region_id, customer_id, COALESCE(channel_id,''), COALESCE(shipping_method_id,''), COALESCE(shipping_address_country,''), COALESCE(shipping_address_postal_code,''), COALESCE(billing_address_country,''), COALESCE(billing_address_postal_code,''), status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
`, in.ID, in.TenantID, in.RegionID, in.CustomerID, in.ChannelID, in.ShippingMethodID, in.ShippingAddressCountry, in.ShippingAddressPostalCode, in.BillingAddressCountry, in.BillingAddressPostalCode, in.Status, in.Currency, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode)
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
    channel_id = NULLIF($8,''),
    shipping_method_id = NULLIF($9,''),
    shipping_address_country = NULLIF($10,''),
    shipping_address_postal_code = NULLIF($11,''),
    billing_address_country = NULLIF($12,''),
    billing_address_postal_code = NULLIF($13,''),
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode, in.ChannelID, in.ShippingMethodID, in.ShippingAddressCountry, in.ShippingAddressPostalCode, in.BillingAddressCountry, in.BillingAddressPostalCode)
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

func (r *PostgresRepository) ResolveShippingMethodPrice(ctx context.Context, tenantID, regionID, shippingMethodID, countryCode, channelID, postalCode, currency string, subtotalCents int64) (int64, bool, error) {
	var price int64
	err := r.db.QueryRowContext(ctx, `
SELECT m.price_cents
FROM shipping_methods m
JOIN shipping_zones z ON z.id = m.shipping_zone_id AND z.tenant_id = m.tenant_id AND z.region_id = m.region_id
WHERE m.tenant_id = $1
  AND m.region_id = $2
  AND m.id = $3
  AND UPPER(m.currency) = UPPER($4)
  AND (
    jsonb_array_length(COALESCE(m.channel_ids, '[]'::jsonb)) = 0
    OR COALESCE(m.channel_ids, '[]'::jsonb) @> to_jsonb(ARRAY[$5]::text[])
  )
  AND (
    jsonb_array_length(COALESCE(m.postal_prefixes, '[]'::jsonb)) = 0
    OR EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(COALESCE(m.postal_prefixes, '[]'::jsonb)) pref(value)
      WHERE UPPER($6) LIKE UPPER(pref.value) || '%'
    )
  )
  AND ($7 >= COALESCE(m.min_order_cents, 0))
  AND ($7 <= COALESCE(m.max_order_cents, 9223372036854775807))
  AND EXISTS (
    SELECT 1
    FROM jsonb_array_elements_text(COALESCE(z.countries, '[]'::jsonb)) country(value)
    WHERE UPPER(country.value) = UPPER($8)
  )
`, tenantID, regionID, shippingMethodID, currency, channelID, postalCode, subtotalCents, countryCode).Scan(&price)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return price, true, nil
}

func (r *PostgresRepository) UpdateShippingCents(ctx context.Context, tenantID, regionID, checkoutID string, shippingCents int64) (Session, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE checkout_sessions
SET shipping_cents = $4,
    total_cents = subtotal_cents + $4,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID, shippingCents)
	if err != nil {
		return Session{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Session{}, ErrSessionNotFound
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) HasAuthorizedPaymentCoverage(ctx context.Context, tenantID, regionID, checkoutID string, requiredTotalCents int64) (bool, error) {
	var covered int64
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(amount_cents), 0)
FROM payments
WHERE tenant_id = $1
  AND region_id = $2
  AND checkout_id = $3
  AND status IN ('authorized', 'partially_captured', 'captured')
`, tenantID, regionID, checkoutID).Scan(&covered)
	if err != nil {
		return false, err
	}
	return covered >= requiredTotalCents && requiredTotalCents > 0, nil
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

func (r *PostgresRepository) GetSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) ListLines(ctx context.Context, tenantID, regionID, checkoutID string) ([]Line, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
FROM checkout_lines
WHERE tenant_id = $1 AND region_id = $2 AND checkout_id = $3
ORDER BY created_at ASC
`, tenantID, regionID, checkoutID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Line{}
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.ID, &l.CheckoutID, &l.ProductID, &l.VariantID, &l.Quantity, &l.UnitPriceCents, &l.Currency); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ChannelIsActive(ctx context.Context, tenantID, regionID, channelID string) (bool, error) {
	var active bool
	err := r.db.QueryRowContext(ctx, `
SELECT is_active
FROM channels
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, channelID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return active, nil
}

func (r *PostgresRepository) GetProductChannelListing(ctx context.Context, tenantID, regionID, channelID, productID string) (bool, bool, error) {
	var isPublished bool
	err := r.db.QueryRowContext(ctx, `
SELECT is_published
FROM product_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3 AND product_id = $4
`, tenantID, regionID, channelID, productID).Scan(&isPublished)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return isPublished, true, nil
}

func (r *PostgresRepository) GetVariantChannelListing(ctx context.Context, tenantID, regionID, channelID, variantID string) (int64, string, bool, bool, error) {
	var priceCents int64
	var currency string
	var isPublished bool
	err := r.db.QueryRowContext(ctx, `
SELECT price_cents, currency, is_published
FROM variant_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3 AND variant_id = $4
`, tenantID, regionID, channelID, variantID).Scan(&priceCents, &currency, &isPublished)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, false, nil
	}
	if err != nil {
		return 0, "", false, false, err
	}
	return priceCents, currency, isPublished, true, nil
}

func (r *PostgresRepository) GetVariantProductID(ctx context.Context, tenantID, regionID, variantID string) (string, bool, error) {
	var productID string
	err := r.db.QueryRowContext(ctx, `
SELECT product_id
FROM product_variants
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, variantID).Scan(&productID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return productID, true, nil
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
SELECT id, tenant_id, region_id, customer_id, COALESCE(channel_id,''), COALESCE(shipping_method_id,''), COALESCE(shipping_address_country,''), COALESCE(shipping_address_postal_code,''), COALESCE(billing_address_country,''), COALESCE(billing_address_postal_code,''), status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
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
	if strings.TrimSpace(session.ChannelID) != "" {
		active, activeErr := channelIsActiveTx(ctx, tx, tenantID, regionID, session.ChannelID)
		if activeErr != nil {
			return OrderCreatedPayload{}, activeErr
		}
		if !active {
			return OrderCreatedPayload{}, ErrChannelListingMismatch
		}
		for _, l := range collected {
			ok, validateErr := lineMatchesChannelListingTx(ctx, tx, tenantID, regionID, session.ChannelID, l)
			if validateErr != nil {
				return OrderCreatedPayload{}, validateErr
			}
			if !ok {
				return OrderCreatedPayload{}, ErrChannelListingMismatch
			}
		}
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

func channelIsActiveTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, channelID string) (bool, error) {
	var active bool
	err := tx.QueryRowContext(ctx, `
SELECT is_active
FROM channels
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, channelID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return active, nil
}

func lineMatchesChannelListingTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, channelID string, line Line) (bool, error) {
	if strings.TrimSpace(line.VariantID) != "" {
		var priceCents int64
		var currency string
		var isPublished bool
		err := tx.QueryRowContext(ctx, `
SELECT price_cents, currency, is_published
FROM variant_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3 AND variant_id = $4
`, tenantID, regionID, channelID, line.VariantID).Scan(&priceCents, &currency, &isPublished)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !isPublished {
			return false, nil
		}
		return line.UnitPriceCents == priceCents && strings.EqualFold(strings.TrimSpace(line.Currency), strings.TrimSpace(currency)), nil
	}
	if strings.TrimSpace(line.ProductID) != "" {
		var isPublished bool
		err := tx.QueryRowContext(ctx, `
SELECT is_published
FROM product_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3 AND product_id = $4
`, tenantID, regionID, channelID, line.ProductID).Scan(&isPublished)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return isPublished, nil
	}
	return true, nil
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
SELECT id, tenant_id, region_id, customer_id, COALESCE(channel_id,''), COALESCE(shipping_method_id,''), COALESCE(shipping_address_country,''), COALESCE(shipping_address_postal_code,''), COALESCE(billing_address_country,''), COALESCE(billing_address_postal_code,''), status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, updated_at
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
		&out.ChannelID,
		&out.ShippingMethodID,
		&out.ShippingAddressCountry,
		&out.ShippingAddressPostalCode,
		&out.BillingAddressCountry,
		&out.BillingAddressPostalCode,
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
