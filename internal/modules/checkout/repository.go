package checkout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbsqlc "rewrite/internal/shared/db/sqlc"
	"rewrite/internal/shared/utils"
)

const checkoutSessionCreateScope = "checkouts.sessions.create"

func checkoutCompleteScope(checkoutID string) string {
	return "checkouts.complete:" + checkoutID
}

func checkoutApplyCustomerAddressesScope(checkoutID string) string {
	return "checkouts.apply_customer_addresses:" + checkoutID
}

func checkoutLineUpsertScope(checkoutID, lineID string) string {
	return "checkouts.line.upsert:" + checkoutID + ":" + lineID
}

func checkoutRecalculateScope(checkoutID string) string {
	return "checkouts.recalculate:" + checkoutID
}

func checkoutPatchSessionScope(checkoutID string) string {
	return "checkouts.session.patch:" + checkoutID
}

func checkoutGiftCardApplyScope(checkoutID string) string {
	return "checkouts.gift_card.apply:" + checkoutID
}

func checkoutGiftCardRemoveScope(checkoutID string) string {
	return "checkouts.gift_card.remove:" + checkoutID
}

// checkoutSessionSelectColumns is the canonical column list for full Session scans (order must match scanSessionInto).
const checkoutSessionSelectColumns = `id, tenant_id, region_id, customer_id, COALESCE(channel_id,''), COALESCE(shipping_method_id,''), COALESCE(shipping_address_country,''), COALESCE(shipping_address_postal_code,''), COALESCE(billing_address_country,''), COALESCE(billing_address_postal_code,''), status, currency, COALESCE(voucher_code,''), COALESCE(promotion_id,''), COALESCE(tax_class_id,''), COALESCE(country_code,''), subtotal_cents, shipping_cents, tax_cents, total_cents, COALESCE(gift_card_id,''), gift_card_applied_cents, updated_at`

var ErrSessionNotFound = errors.New("checkout session not found")
var ErrSessionNotOpen = errors.New("checkout session is not open")
var ErrCheckoutEmpty = errors.New("checkout has no lines")
var ErrInsufficientStock = errors.New("insufficient stock for checkout line")
var ErrVoucherUnavailable = errors.New("voucher is unavailable")
var ErrChannelListingMismatch = errors.New("checkout line no longer matches channel listing")
var ErrShippingAddressCountryRequired = errors.New("shipping_address_country is required when shipping_method_id is set")
var ErrShippingMethodNotEligible = errors.New("selected shipping method is not eligible for checkout context")
var ErrCustomerAddressNotApplicable = errors.New("customer address not found or does not belong to this checkout customer")
var ErrIdempotencyKeyRequired = errors.New("checkout idempotency key is required")
var ErrCheckoutIdempotencyOrphan = errors.New("idempotency record references missing checkout session")
var ErrCheckoutCompleteIdempotencyKeyRequired = errors.New("checkout completion idempotency key is required")
var ErrCheckoutCompleteIdempotencyOrphan = errors.New("checkout completion idempotency references missing order")
var ErrCheckoutApplyAddressesIdempotencyKeyRequired = errors.New("apply customer addresses idempotency key is required")
var ErrCheckoutApplyAddressesIdempotencyOrphan = errors.New("apply customer addresses idempotency references missing checkout session")
var ErrCheckoutApplyAddressesIdempotencyMismatch = errors.New("apply customer addresses idempotency record mismatch")
var ErrCheckoutLineUpsertIdempotencyKeyRequired = errors.New("checkout line upsert idempotency key is required")
var ErrCheckoutLineUpsertIdempotencyOrphan = errors.New("checkout line upsert idempotency references missing line")
var ErrCheckoutLineUpsertIdempotencyMismatch = errors.New("checkout line upsert idempotency record mismatch")
var ErrCheckoutRecalculateIdempotencyOrphan = errors.New("checkout recalculate idempotency references missing checkout session")
var ErrCheckoutRecalculateIdempotencyMismatch = errors.New("checkout recalculate idempotency record mismatch")
var ErrCheckoutPatchSessionIdempotencyKeyRequired = errors.New("checkout session patch idempotency key is required")
var ErrCheckoutPatchSessionIdempotencyOrphan = errors.New("checkout session patch idempotency references missing checkout session")
var ErrCheckoutPatchSessionIdempotencyMismatch = errors.New("checkout session patch idempotency record mismatch")
var ErrCheckoutGiftCardApplyIdempotencyKeyRequired = errors.New("checkout gift card apply idempotency key is required")
var ErrCheckoutGiftCardApplyIdempotencyOrphan = errors.New("checkout gift card apply idempotency references missing checkout session")
var ErrCheckoutGiftCardApplyIdempotencyMismatch = errors.New("checkout gift card apply idempotency record mismatch")
var ErrCheckoutGiftCardRemoveIdempotencyKeyRequired = errors.New("checkout gift card remove idempotency key is required")
var ErrCheckoutGiftCardRemoveIdempotencyOrphan = errors.New("checkout gift card remove idempotency references missing checkout session")
var ErrCheckoutGiftCardRemoveIdempotencyMismatch = errors.New("checkout gift card remove idempotency record mismatch")
var ErrGiftCardNotFound = errors.New("gift card not found")
var ErrGiftCardInactive = errors.New("gift card is inactive")
var ErrGiftCardExpired = errors.New("gift card has expired")
var ErrGiftCardCurrencyMismatch = errors.New("gift card currency does not match checkout")
var ErrGiftCardInUse = errors.New("gift card is already applied to another open checkout")
var ErrGiftCardDepleted = errors.New("gift card balance is insufficient to complete checkout")

// RecalculateOptions configures transactional checkout recalculation when a pricing engine is available.
type RecalculateOptions struct {
	ComputePricing func(ctx context.Context, session Session, baseAmountCents int64) (taxCents, totalCents int64, err error)
}

type Repository interface {
	CreateSession(ctx context.Context, in Session, idempotencyKey string) (Session, error)
	UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session, idempotencyKey string) (Session, error)
	ApplyCustomerAddressesToCheckout(ctx context.Context, tenantID, regionID, checkoutID, shippingAddressID, billingAddressID, idempotencyKey string) (Session, error)
	UpsertLine(ctx context.Context, tenantID, regionID string, line Line, idempotencyKey string) (Line, error)
	GetSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error)
	ListLines(ctx context.Context, tenantID, regionID, checkoutID string) ([]Line, error)
	ChannelIsActive(ctx context.Context, tenantID, regionID, channelID string) (bool, error)
	GetProductChannelListing(ctx context.Context, tenantID, regionID, channelID, productID string) (bool, bool, error)
	GetVariantChannelListing(ctx context.Context, tenantID, regionID, channelID, variantID string) (int64, string, bool, bool, error)
	GetVariantProductID(ctx context.Context, tenantID, regionID, variantID string) (string, bool, error)
	ResolveShippingMethodPrice(ctx context.Context, tenantID, regionID, shippingMethodID, countryCode, channelID, postalCode, currency string, subtotalCents int64) (int64, bool, error)
	UpdateShippingCents(ctx context.Context, tenantID, regionID, checkoutID string, shippingCents int64) (Session, error)
	HasAuthorizedPaymentCoverage(ctx context.Context, tenantID, regionID, checkoutID string, requiredTotalCents int64) (bool, error)
	ValidateCheckoutStock(ctx context.Context, tenantID, regionID, checkoutID string) error
	Recalculate(ctx context.Context, tenantID, regionID, checkoutID string, opts *RecalculateOptions, idempotencyKey string) (Session, error)
	UpdatePricing(ctx context.Context, tenantID, regionID, checkoutID string, taxCents, totalCents int64) (Session, error)
	ValidateGiftCardForSession(ctx context.Context, tenantID, regionID string, session Session) error
	ApplyGiftCardToCheckout(ctx context.Context, tenantID, regionID, checkoutID, code, idempotencyKey string) (Session, error)
	RemoveGiftCardFromCheckout(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (Session, error)
	Complete(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (CompleteOutcome, error)
}

type PostgresRepository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn, queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) CreateSession(ctx context.Context, in Session, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := in.TenantID + "\x00" + checkoutSessionCreateScope + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Session{}, err
	}

	qtx := r.queries.WithTx(tx)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       in.TenantID,
		Scope:          checkoutSessionCreateScope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, resourceID, in.TenantID, in.RegionID)
		sess, scanErr := scanSession(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Session{}, fmt.Errorf("%w: %q", ErrCheckoutIdempotencyOrphan, resourceID)
		}
		if scanErr != nil {
			return Session{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return sess, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Session{}, idemErr
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO checkout_sessions (id, tenant_id, region_id, customer_id, channel_id, shipping_method_id, shipping_address_country, shipping_address_postal_code, billing_address_country, billing_address_postal_code, status, currency, voucher_code, promotion_id, tax_class_id, country_code, subtotal_cents, shipping_cents, tax_cents, total_cents, gift_card_id, gift_card_applied_cents, created_at, updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),0,0,0,0,NULL,0,NOW(),NOW())
RETURNING `+checkoutSessionSelectColumns+`
`, in.ID, in.TenantID, in.RegionID, in.CustomerID, in.ChannelID, in.ShippingMethodID, in.ShippingAddressCountry, in.ShippingAddressPostalCode, in.BillingAddressCountry, in.BillingAddressPostalCode, in.Status, in.Currency, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode)
	saved, err := scanSession(row)
	if err != nil {
		return Session{}, err
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       in.TenantID,
		Scope:          checkoutSessionCreateScope,
		IdempotencyKey: key,
		ResourceID:     saved.ID,
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return saved, nil
}

func (r *PostgresRepository) UpsertLine(ctx context.Context, tenantID, regionID string, line Line, idempotencyKey string) (Line, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Line{}, ErrCheckoutLineUpsertIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Line{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := tenantID + "\x00" + checkoutLineUpsertScope(line.CheckoutID, line.ID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Line{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutLineUpsertScope(line.CheckoutID, line.ID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != line.ID {
			return Line{}, ErrCheckoutLineUpsertIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `
SELECT id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
FROM checkout_lines
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND checkout_id = $4
`, line.ID, tenantID, regionID, line.CheckoutID)
		out, scanErr := scanLine(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Line{}, fmt.Errorf("%w: %q", ErrCheckoutLineUpsertIdempotencyOrphan, line.ID)
		}
		if scanErr != nil {
			return Line{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Line{}, err
		}
		return out, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Line{}, idemErr
	}

	if err := lockCheckoutSessionOpenTx(ctx, tx, tenantID, regionID, line.CheckoutID); err != nil {
		return Line{}, err
	}

	stockItemID, err := resolveStockItemID(ctx, tx, tenantID, regionID, line)
	if err != nil {
		return Line{}, err
	}

	var existingResID, existingStockID string
	exErr := tx.QueryRowContext(ctx, `
SELECT id, stock_item_id
FROM stock_reservations
WHERE tenant_id = $1 AND region_id = $2 AND checkout_id = $3 AND checkout_line_id = $4
FOR UPDATE
`, tenantID, regionID, line.CheckoutID, line.ID).Scan(&existingResID, &existingStockID)
	hasExisting := exErr == nil
	if exErr != nil && !errors.Is(exErr, sql.ErrNoRows) {
		return Line{}, exErr
	}

	lockIDs := []string{stockItemID}
	if hasExisting && existingStockID != stockItemID {
		lockIDs = append(lockIDs, existingStockID)
	}
	lockIDs = sortedUniqueNonEmpty(lockIDs)

	quantities := make(map[string]int64, len(lockIDs))
	for _, sid := range lockIDs {
		var q int64
		serr := tx.QueryRowContext(ctx, `
SELECT quantity FROM stock_items WHERE tenant_id = $1 AND region_id = $2 AND id = $3 FOR UPDATE
`, tenantID, regionID, sid).Scan(&q)
		if errors.Is(serr, sql.ErrNoRows) {
			return Line{}, ErrInsufficientStock
		}
		if serr != nil {
			return Line{}, serr
		}
		quantities[sid] = q
	}

	if hasExisting && existingStockID != stockItemID {
		if _, derr := tx.ExecContext(ctx, `DELETE FROM stock_reservations WHERE id = $1`, existingResID); derr != nil {
			return Line{}, derr
		}
	}

	var sumOther int64
	if serr := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(quantity), 0)
FROM stock_reservations
WHERE tenant_id = $1 AND region_id = $2 AND stock_item_id = $3
  AND expires_at > NOW()
  AND NOT (checkout_id = $4 AND checkout_line_id = $5)
`, tenantID, regionID, stockItemID, line.CheckoutID, line.ID).Scan(&sumOther); serr != nil {
		return Line{}, serr
	}

	onHand := quantities[stockItemID]
	if sumOther+line.Quantity > onHand {
		return Line{}, ErrInsufficientStock
	}

	if hasExisting && existingStockID == stockItemID {
		if _, derr := tx.ExecContext(ctx, `DELETE FROM stock_reservations WHERE id = $1`, existingResID); derr != nil {
			return Line{}, derr
		}
	}

	if _, ierr := tx.ExecContext(ctx, `
INSERT INTO stock_reservations (id, tenant_id, region_id, checkout_id, checkout_line_id, stock_item_id, quantity, expires_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7, NOW() + INTERVAL '168 hours', NOW(), NOW())
`, utils.NewID("rsv"), tenantID, regionID, line.CheckoutID, line.ID, stockItemID, line.Quantity); ierr != nil {
		return Line{}, ierr
	}

	if line.ID != "" {
		row := tx.QueryRowContext(ctx, `
UPDATE checkout_lines
SET product_id = NULLIF($6,''), variant_id = NULLIF($7,''), quantity = $4, unit_price_cents = $5, currency = $8, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND checkout_id = $9
  AND EXISTS (
    SELECT 1 FROM checkout_sessions s
    WHERE s.id = $9 AND s.tenant_id = $2 AND s.region_id = $3 AND s.status = 'open'
  )
RETURNING id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
`, line.ID, tenantID, regionID, line.Quantity, line.UnitPriceCents, line.ProductID, line.VariantID, line.Currency, line.CheckoutID)
		out, lerr := scanLine(row)
		if lerr == nil {
			if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
				TenantID:       tenantID,
				Scope:          scope,
				IdempotencyKey: key,
				ResourceID:     out.ID,
			}); err != nil {
				return Line{}, err
			}
			if cerr := tx.Commit(); cerr != nil {
				return Line{}, cerr
			}
			return out, nil
		}
		if !errors.Is(lerr, sql.ErrNoRows) {
			return Line{}, lerr
		}
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO checkout_lines (id, tenant_id, region_id, checkout_id, product_id, variant_id, quantity, unit_price_cents, currency, created_at, updated_at)
SELECT $1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,NOW(),NOW()
WHERE EXISTS (
  SELECT 1 FROM checkout_sessions s
  WHERE s.id = $4 AND s.tenant_id = $2 AND s.region_id = $3 AND s.status = 'open'
)
RETURNING id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
`, line.ID, tenantID, regionID, line.CheckoutID, line.ProductID, line.VariantID, line.Quantity, line.UnitPriceCents, line.Currency)
	out, err := scanLine(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status, found, statusErr := r.sessionStatus(ctx, tenantID, regionID, line.CheckoutID)
			if statusErr != nil {
				return Line{}, statusErr
			}
			if !found {
				return Line{}, ErrSessionNotFound
			}
			if status != "open" {
				return Line{}, ErrSessionNotOpen
			}
		}
		return Line{}, err
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     out.ID,
	}); err != nil {
		return Line{}, err
	}
	if cerr := tx.Commit(); cerr != nil {
		return Line{}, cerr
	}
	return out, nil
}

func lockCheckoutSessionOpenTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, checkoutID string) error {
	var status string
	err := tx.QueryRowContext(ctx, `
SELECT status FROM checkout_sessions WHERE id = $1 AND tenant_id = $2 AND region_id = $3 FOR UPDATE
`, checkoutID, tenantID, regionID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return ErrSessionNotOpen
	}
	return nil
}

func sortedUniqueNonEmpty(ss []string) []string {
	seen := make(map[string]struct{})
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func (r *PostgresRepository) UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutPatchSessionIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := tenantID + "\x00" + checkoutPatchSessionScope(checkoutID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Session{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutPatchSessionScope(checkoutID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != checkoutID {
			return Session{}, ErrCheckoutPatchSessionIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		sess, scanErr := scanSession(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Session{}, fmt.Errorf("%w: %q", ErrCheckoutPatchSessionIdempotencyOrphan, checkoutID)
		}
		if scanErr != nil {
			return Session{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return sess, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Session{}, idemErr
	}

	res, err := tx.ExecContext(ctx, `
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
  AND status = 'open'
`, checkoutID, tenantID, regionID, in.VoucherCode, in.PromotionID, in.TaxClassID, in.CountryCode, in.ChannelID, in.ShippingMethodID, in.ShippingAddressCountry, in.ShippingAddressPostalCode, in.BillingAddressCountry, in.BillingAddressPostalCode)
	if err != nil {
		return Session{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		var st string
		serr := tx.QueryRowContext(ctx, `
SELECT status FROM checkout_sessions WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID).Scan(&st)
		if errors.Is(serr, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		if serr != nil {
			return Session{}, serr
		}
		return Session{}, ErrSessionNotOpen
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     checkoutID,
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) ApplyCustomerAddressesToCheckout(ctx context.Context, tenantID, regionID, checkoutID, shippingAddressID, billingAddressID, idempotencyKey string) (Session, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutApplyAddressesIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := tenantID + "\x00" + checkoutApplyCustomerAddressesScope(checkoutID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Session{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutApplyCustomerAddressesScope(checkoutID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != checkoutID {
			return Session{}, ErrCheckoutApplyAddressesIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		sess, scanErr := scanSession(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Session{}, fmt.Errorf("%w: %q", ErrCheckoutApplyAddressesIdempotencyOrphan, checkoutID)
		}
		if scanErr != nil {
			return Session{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return sess, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Session{}, idemErr
	}

	var customerID, status string
	var shipC, shipP, billC, billP string
	err = tx.QueryRowContext(ctx, `
SELECT customer_id, status,
  COALESCE(shipping_address_country,''), COALESCE(shipping_address_postal_code,''),
  COALESCE(billing_address_country,''), COALESCE(billing_address_postal_code,'')
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, checkoutID, tenantID, regionID).Scan(&customerID, &status, &shipC, &shipP, &billC, &billP)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if status != "open" {
		return Session{}, ErrSessionNotOpen
	}

	shipID := strings.TrimSpace(shippingAddressID)
	billID := strings.TrimSpace(billingAddressID)

	finalShipC, finalShipP := shipC, shipP
	finalBillC, finalBillP := billC, billP

	if shipID != "" {
		serr := tx.QueryRowContext(ctx, `
SELECT UPPER(country_code), TRIM(postal_code)
FROM customer_addresses
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND customer_id = $4
`, shipID, tenantID, regionID, customerID).Scan(&finalShipC, &finalShipP)
		if errors.Is(serr, sql.ErrNoRows) {
			return Session{}, ErrCustomerAddressNotApplicable
		}
		if serr != nil {
			return Session{}, serr
		}
	}
	if billID != "" {
		berr := tx.QueryRowContext(ctx, `
SELECT UPPER(country_code), TRIM(postal_code)
FROM customer_addresses
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND customer_id = $4
`, billID, tenantID, regionID, customerID).Scan(&finalBillC, &finalBillP)
		if errors.Is(berr, sql.ErrNoRows) {
			return Session{}, ErrCustomerAddressNotApplicable
		}
		if berr != nil {
			return Session{}, berr
		}
	}

	res, uerr := tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET shipping_address_country = NULLIF($4,''),
    shipping_address_postal_code = NULLIF($5,''),
    billing_address_country = NULLIF($6,''),
    billing_address_postal_code = NULLIF($7,''),
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID, finalShipC, finalShipP, finalBillC, finalBillP)
	if uerr != nil {
		return Session{}, uerr
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return Session{}, raErr
	}
	if n == 0 {
		return Session{}, ErrSessionNotOpen
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     checkoutID,
	}); err != nil {
		return Session{}, err
	}
	if cerr := tx.Commit(); cerr != nil {
		return Session{}, cerr
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) Recalculate(ctx context.Context, tenantID, regionID, checkoutID string, opts *RecalculateOptions, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	qtx := r.queries.WithTx(tx)
	if key != "" {
		lockKey := tenantID + "\x00" + checkoutRecalculateScope(checkoutID) + "\x00" + key
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
			return Session{}, err
		}
		scope := checkoutRecalculateScope(checkoutID)
		resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
			TenantID:       tenantID,
			Scope:          scope,
			IdempotencyKey: key,
		})
		if idemErr == nil && resourceID != "" {
			if resourceID != checkoutID {
				return Session{}, ErrCheckoutRecalculateIdempotencyMismatch
			}
			row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
			sess, scanErr := scanSession(row)
			if errors.Is(scanErr, sql.ErrNoRows) {
				return Session{}, fmt.Errorf("%w: %q", ErrCheckoutRecalculateIdempotencyOrphan, checkoutID)
			}
			if scanErr != nil {
				return Session{}, scanErr
			}
			if err := tx.Commit(); err != nil {
				return Session{}, err
			}
			return sess, nil
		}
		if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
			return Session{}, idemErr
		}
	}

	lockRow := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
FOR UPDATE
`, checkoutID, tenantID, regionID)
	var locked Session
	if err := scanSessionInto(lockRow, &locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status, found, sErr := r.sessionStatus(ctx, tenantID, regionID, checkoutID)
			if sErr != nil {
				return Session{}, sErr
			}
			if !found {
				return Session{}, ErrSessionNotFound
			}
			if status != "open" {
				return Session{}, ErrSessionNotOpen
			}
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET subtotal_cents = COALESCE((SELECT SUM(quantity * unit_price_cents) FROM checkout_lines WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3), 0),
    total_cents = COALESCE((SELECT SUM(quantity * unit_price_cents) FROM checkout_lines WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3), 0)
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID); err != nil {
		return Session{}, err
	}

	if opts == nil || opts.ComputePricing == nil {
		row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		var out Session
		if err := scanSessionInto(row, &out); err != nil {
			return Session{}, err
		}
		if err := finalizeGiftCardTotalsCheckoutTx(ctx, tx, tenantID, regionID, checkoutID, out.TotalCents, out.Currency, locked.GiftCardID); err != nil {
			return Session{}, err
		}
		rowFin := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		if err := scanSessionInto(rowFin, &out); err != nil {
			return Session{}, err
		}
		if key != "" {
			if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
				TenantID:       tenantID,
				Scope:          checkoutRecalculateScope(checkoutID),
				IdempotencyKey: key,
				ResourceID:     checkoutID,
			}); err != nil {
				return Session{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return out, nil
	}

	row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	var session Session
	if err := scanSessionInto(row, &session); err != nil {
		return Session{}, err
	}

	var shippingCents int64
	if strings.TrimSpace(session.ShippingMethodID) != "" {
		if strings.TrimSpace(session.ShippingAddressCountry) == "" {
			return Session{}, ErrShippingAddressCountryRequired
		}
		price, ok, sErr := resolveShippingMethodPriceTx(ctx, tx, tenantID, regionID, session.ShippingMethodID, session.ShippingAddressCountry, session.ChannelID, session.ShippingAddressPostalCode, session.Currency, session.SubtotalCents)
		if sErr != nil {
			return Session{}, sErr
		}
		if !ok {
			return Session{}, ErrShippingMethodNotEligible
		}
		shippingCents = price
	} else if session.ShippingCents != 0 {
		shippingCents = 0
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET shipping_cents = $4,
    total_cents = subtotal_cents + $4,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID, shippingCents); err != nil {
		return Session{}, err
	}

	row2 := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	if err := scanSessionInto(row2, &session); err != nil {
		return Session{}, err
	}

	baseAmount := session.SubtotalCents + session.ShippingCents
	taxCents, totalCents, pErr := opts.ComputePricing(ctx, session, baseAmount)
	if pErr != nil {
		return Session{}, pErr
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET tax_cents = $4, total_cents = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID, taxCents, totalCents); err != nil {
		return Session{}, err
	}

	row3 := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	var out Session
	if err := scanSessionInto(row3, &out); err != nil {
		return Session{}, err
	}
	if err := finalizeGiftCardTotalsCheckoutTx(ctx, tx, tenantID, regionID, checkoutID, out.TotalCents, out.Currency, locked.GiftCardID); err != nil {
		return Session{}, err
	}
	row4 := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	var final Session
	if err := scanSessionInto(row4, &final); err != nil {
		return Session{}, err
	}
	if key != "" {
		if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
			TenantID:       tenantID,
			Scope:          checkoutRecalculateScope(checkoutID),
			IdempotencyKey: key,
			ResourceID:     checkoutID,
		}); err != nil {
			return Session{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return final, nil
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

func resolveShippingMethodPriceTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, shippingMethodID, countryCode, channelID, postalCode, currency string, subtotalCents int64) (int64, bool, error) {
	var price int64
	err := tx.QueryRowContext(ctx, `
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
  AND status = 'open'
`, checkoutID, tenantID, regionID, shippingCents)
	if err != nil {
		return Session{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		status, found, statusErr := r.sessionStatus(ctx, tenantID, regionID, checkoutID)
		if statusErr != nil {
			return Session{}, statusErr
		}
		if !found {
			return Session{}, ErrSessionNotFound
		}
		if status != "open" {
			return Session{}, ErrSessionNotOpen
		}
		return Session{}, ErrSessionNotFound
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func finalizeGiftCardTotalsCheckoutTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, checkoutID string, preGiftTotalCents int64, sessionCurrency, giftCardID string) error {
	gcID := strings.TrimSpace(giftCardID)
	if gcID == "" {
		_, err := tx.ExecContext(ctx, `
UPDATE checkout_sessions SET gift_card_applied_cents = 0, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID)
		return err
	}
	var bal int64
	var active bool
	var gcCur string
	var exp sql.NullTime
	err := tx.QueryRowContext(ctx, `
SELECT balance_cents, is_active, currency, expires_at
FROM gift_cards
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, gcID, tenantID, regionID).Scan(&bal, &active, &gcCur, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGiftCardNotFound
	}
	if err != nil {
		return err
	}
	if !active {
		return ErrGiftCardInactive
	}
	if exp.Valid && time.Now().UTC().After(exp.Time.UTC()) {
		return ErrGiftCardExpired
	}
	if !strings.EqualFold(strings.TrimSpace(gcCur), strings.TrimSpace(sessionCurrency)) {
		return ErrGiftCardCurrencyMismatch
	}
	preGift := preGiftTotalCents
	if preGift < 0 {
		preGift = 0
	}
	applied := preGift
	if bal < applied {
		applied = bal
	}
	newTotal := preGift - applied
	if newTotal < 0 {
		newTotal = 0
	}
	_, err = tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET gift_card_applied_cents = $4, total_cents = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID, applied, newTotal)
	return err
}

func (r *PostgresRepository) ValidateGiftCardForSession(ctx context.Context, tenantID, regionID string, session Session) error {
	gcID := strings.TrimSpace(session.GiftCardID)
	if gcID == "" {
		return nil
	}
	var bal int64
	var active bool
	var gcCur string
	var exp sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT balance_cents, is_active, currency, expires_at
FROM gift_cards
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, gcID, tenantID, regionID).Scan(&bal, &active, &gcCur, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGiftCardNotFound
	}
	if err != nil {
		return err
	}
	if !active {
		return ErrGiftCardInactive
	}
	if exp.Valid && time.Now().UTC().After(exp.Time.UTC()) {
		return ErrGiftCardExpired
	}
	if !strings.EqualFold(strings.TrimSpace(gcCur), strings.TrimSpace(session.Currency)) {
		return ErrGiftCardCurrencyMismatch
	}
	if bal <= 0 {
		return ErrGiftCardDepleted
	}
	return nil
}

func (r *PostgresRepository) ApplyGiftCardToCheckout(ctx context.Context, tenantID, regionID, checkoutID, code, idempotencyKey string) (Session, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutGiftCardApplyIdempotencyKeyRequired
	}
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return Session{}, ErrGiftCardNotFound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := tenantID + "\x00" + checkoutGiftCardApplyScope(checkoutID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Session{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutGiftCardApplyScope(checkoutID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != checkoutID {
			return Session{}, ErrCheckoutGiftCardApplyIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		out, scanErr := scanSession(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Session{}, fmt.Errorf("%w: %q", ErrCheckoutGiftCardApplyIdempotencyOrphan, checkoutID)
		}
		if scanErr != nil {
			return Session{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return out, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Session{}, idemErr
	}

	row := tx.QueryRowContext(ctx, `SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
FOR UPDATE
`, checkoutID, tenantID, regionID)
	var sess Session
	if err := scanSessionInto(row, &sess); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}

	var cardID string
	var bal int64
	var active bool
	var gcCur string
	var exp sql.NullTime
	err = tx.QueryRowContext(ctx, `
SELECT id, balance_cents, is_active, currency, expires_at
FROM gift_cards
WHERE tenant_id = $1 AND region_id = $2 AND code = $3
FOR UPDATE
`, tenantID, regionID, normalized).Scan(&cardID, &bal, &active, &gcCur, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrGiftCardNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if !active {
		return Session{}, ErrGiftCardInactive
	}
	if exp.Valid && time.Now().UTC().After(exp.Time.UTC()) {
		return Session{}, ErrGiftCardExpired
	}
	if bal <= 0 {
		return Session{}, ErrGiftCardDepleted
	}
	if !strings.EqualFold(strings.TrimSpace(gcCur), strings.TrimSpace(sess.Currency)) {
		return Session{}, ErrGiftCardCurrencyMismatch
	}

	var otherID string
	oerr := tx.QueryRowContext(ctx, `
SELECT id FROM checkout_sessions
WHERE gift_card_id = $1 AND status = 'open' AND id <> $2 AND tenant_id = $3 AND region_id = $4
LIMIT 1
`, cardID, checkoutID, tenantID, regionID).Scan(&otherID)
	if oerr == nil {
		return Session{}, ErrGiftCardInUse
	}
	if oerr != nil && !errors.Is(oerr, sql.ErrNoRows) {
		return Session{}, oerr
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET gift_card_id = $4, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID, cardID); err != nil {
		return Session{}, err
	}

	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     checkoutID,
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return r.getSession(ctx, tenantID, regionID, checkoutID)
}

func (r *PostgresRepository) RemoveGiftCardFromCheckout(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (Session, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutGiftCardRemoveIdempotencyKeyRequired
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback() }()
	lockKey := tenantID + "\x00" + checkoutGiftCardRemoveScope(checkoutID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Session{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutGiftCardRemoveScope(checkoutID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != checkoutID {
			return Session{}, ErrCheckoutGiftCardRemoveIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
		out, scanErr := scanSession(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Session{}, fmt.Errorf("%w: %q", ErrCheckoutGiftCardRemoveIdempotencyOrphan, checkoutID)
		}
		if scanErr != nil {
			return Session{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return out, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Session{}, idemErr
	}
	res, err := tx.ExecContext(ctx, `
UPDATE checkout_sessions
SET gift_card_id = NULL, gift_card_applied_cents = 0, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND status = 'open'
`, checkoutID, tenantID, regionID)
	if err != nil {
		return Session{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var st string
		serr := tx.QueryRowContext(ctx, `SELECT status FROM checkout_sessions WHERE id = $1 AND tenant_id = $2 AND region_id = $3`, checkoutID, tenantID, regionID).Scan(&st)
		if errors.Is(serr, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		if serr != nil {
			return Session{}, serr
		}
		return Session{}, ErrSessionNotOpen
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     checkoutID,
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
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
	if requiredTotalCents <= 0 {
		return true, nil
	}
	return covered >= requiredTotalCents, nil
}

func (r *PostgresRepository) UpdatePricing(ctx context.Context, tenantID, regionID, checkoutID string, taxCents, totalCents int64) (Session, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE checkout_sessions
SET tax_cents = $4, total_cents = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
  AND status = 'open'
`, checkoutID, tenantID, regionID, taxCents, totalCents)
	if err != nil {
		return Session{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		status, found, statusErr := r.sessionStatus(ctx, tenantID, regionID, checkoutID)
		if statusErr != nil {
			return Session{}, statusErr
		}
		if !found {
			return Session{}, ErrSessionNotFound
		}
		if status != "open" {
			return Session{}, ErrSessionNotOpen
		}
		return Session{}, ErrSessionNotFound
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

func (r *PostgresRepository) Complete(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (CompleteOutcome, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return CompleteOutcome{}, ErrCheckoutCompleteIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CompleteOutcome{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := tenantID + "\x00" + checkoutCompleteScope(checkoutID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return CompleteOutcome{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := checkoutCompleteScope(checkoutID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, COALESCE(checkout_id, '')
FROM orders
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND checkout_id = $4
`, resourceID, tenantID, regionID, checkoutID)
		var p OrderCreatedPayload
		if scanErr := row.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.CustomerID, &p.Status, &p.TotalCents, &p.Currency, &p.CheckoutID); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return CompleteOutcome{}, fmt.Errorf("%w: %q", ErrCheckoutCompleteIdempotencyOrphan, resourceID)
			}
			return CompleteOutcome{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return CompleteOutcome{}, err
		}
		return CompleteOutcome{Payload: p, FromIdempotencyReplay: true}, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return CompleteOutcome{}, idemErr
	}

	orderID := utils.NewID("ord")

	var session Session
	row := tx.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, checkoutID, tenantID, regionID)
	if scanErr := scanSessionInto(row, &session); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return CompleteOutcome{}, ErrSessionNotFound
		}
		return CompleteOutcome{}, scanErr
	}
	if session.Status != "open" {
		return CompleteOutcome{}, ErrSessionNotOpen
	}

	lines, err := tx.QueryContext(ctx, `
SELECT id, checkout_id, COALESCE(product_id,''), COALESCE(variant_id,''), quantity, unit_price_cents, currency
FROM checkout_lines
WHERE checkout_id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID)
	if err != nil {
		return CompleteOutcome{}, err
	}
	defer func() {
		_ = lines.Close()
	}()
	collected := make([]Line, 0)
	for lines.Next() {
		var l Line
		if err = lines.Scan(&l.ID, &l.CheckoutID, &l.ProductID, &l.VariantID, &l.Quantity, &l.UnitPriceCents, &l.Currency); err != nil {
			return CompleteOutcome{}, err
		}
		collected = append(collected, l)
	}
	if len(collected) == 0 {
		return CompleteOutcome{}, ErrCheckoutEmpty
	}
	if err = lines.Err(); err != nil {
		return CompleteOutcome{}, err
	}
	if strings.TrimSpace(session.ChannelID) != "" {
		active, activeErr := channelIsActiveTx(ctx, tx, tenantID, regionID, session.ChannelID)
		if activeErr != nil {
			return CompleteOutcome{}, activeErr
		}
		if !active {
			return CompleteOutcome{}, ErrChannelListingMismatch
		}
		for _, l := range collected {
			ok, validateErr := lineMatchesChannelListingTx(ctx, tx, tenantID, regionID, session.ChannelID, l)
			if validateErr != nil {
				return CompleteOutcome{}, validateErr
			}
			if !ok {
				return CompleteOutcome{}, ErrChannelListingMismatch
			}
		}
	}

	// Wave 1 invariant: reserve stock atomically during checkout completion.
	requiredByStockItem := map[string]int64{}
	for _, l := range collected {
		stockItemID, resolveErr := resolveStockItemID(ctx, tx, tenantID, regionID, l)
		if resolveErr != nil {
			return CompleteOutcome{}, resolveErr
		}
		requiredByStockItem[stockItemID] += l.Quantity
	}
	for stockItemID, required := range requiredByStockItem {
		var onHand int64
		row := tx.QueryRowContext(ctx, `
SELECT quantity
FROM stock_items
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
FOR UPDATE
`, tenantID, regionID, stockItemID)
		if scanErr := row.Scan(&onHand); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return CompleteOutcome{}, ErrInsufficientStock
			}
			return CompleteOutcome{}, scanErr
		}
		var othersReserved int64
		if scanErr := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(quantity), 0)
FROM stock_reservations
WHERE tenant_id = $1 AND region_id = $2 AND stock_item_id = $3
  AND checkout_id <> $4
  AND expires_at > NOW()
`, tenantID, regionID, stockItemID, checkoutID).Scan(&othersReserved); scanErr != nil {
			return CompleteOutcome{}, scanErr
		}
		if onHand < othersReserved+required {
			return CompleteOutcome{}, ErrInsufficientStock
		}
		if _, err = tx.ExecContext(ctx, `
UPDATE stock_items
SET quantity = quantity - $4, updated_at = NOW()
WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, stockItemID, required); err != nil {
			return CompleteOutcome{}, err
		}
		if _, err = tx.ExecContext(ctx, `
DELETE FROM stock_reservations
WHERE tenant_id = $1 AND region_id = $2 AND checkout_id = $3 AND stock_item_id = $4
`, tenantID, regionID, checkoutID, stockItemID); err != nil {
			return CompleteOutcome{}, err
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO stock_allocations (id, tenant_id, region_id, order_id, checkout_id, stock_item_id, quantity, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
`, utils.NewID("alc"), tenantID, regionID, orderID, checkoutID, stockItemID, required); err != nil {
			return CompleteOutcome{}, err
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
				return CompleteOutcome{}, ErrVoucherUnavailable
			}
			return CompleteOutcome{}, scanErr
		}
	}

	if strings.TrimSpace(session.GiftCardID) != "" && session.GiftCardAppliedCents > 0 {
		res, gerr := tx.ExecContext(ctx, `
UPDATE gift_cards
SET balance_cents = balance_cents - $4, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND balance_cents >= $4
`, session.GiftCardID, tenantID, regionID, session.GiftCardAppliedCents)
		if gerr != nil {
			return CompleteOutcome{}, gerr
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return CompleteOutcome{}, raErr
		}
		if n == 0 {
			return CompleteOutcome{}, ErrGiftCardDepleted
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO orders (id, tenant_id, region_id, customer_id, status, total_cents, currency, checkout_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,'created',$5,$6,$7,NOW(),NOW())
`, orderID, tenantID, regionID, session.CustomerID, session.TotalCents, session.Currency, session.ID); err != nil {
		return CompleteOutcome{}, err
	}
	for _, l := range collected {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO order_lines (id, tenant_id, region_id, order_id, product_id, variant_id, quantity, unit_price_cents, total_cents, currency, created_at, updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,NOW(),NOW())
`, "ol_"+l.ID, tenantID, regionID, orderID, l.ProductID, l.VariantID, l.Quantity, l.UnitPriceCents, l.Quantity*l.UnitPriceCents, l.Currency); err != nil {
			return CompleteOutcome{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE payments
SET order_id = $4,
    updated_at = NOW()
WHERE tenant_id = $1
  AND region_id = $2
  AND checkout_id = $3
  AND status IN ('authorized', 'partially_captured', 'captured')
`, tenantID, regionID, checkoutID, orderID); err != nil {
		return CompleteOutcome{}, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE checkout_sessions SET status = 'completed', updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID); err != nil {
		return CompleteOutcome{}, err
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     orderID,
	}); err != nil {
		return CompleteOutcome{}, err
	}
	if err = tx.Commit(); err != nil {
		return CompleteOutcome{}, err
	}
	return CompleteOutcome{
		Payload: OrderCreatedPayload{
			ID:         orderID,
			TenantID:   tenantID,
			RegionID:   regionID,
			CustomerID: session.CustomerID,
			Status:     "created",
			TotalCents: session.TotalCents,
			Currency:   session.Currency,
			CheckoutID: session.ID,
		},
		FromIdempotencyReplay: false,
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

func resolveStockItemIDDB(ctx context.Context, db *sql.DB, tenantID, regionID string, line Line) (string, error) {
	var stockItemID string
	if line.VariantID != "" {
		row := db.QueryRowContext(ctx, `
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
		row := db.QueryRowContext(ctx, `
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

func (r *PostgresRepository) ValidateCheckoutStock(ctx context.Context, tenantID, regionID, checkoutID string) error {
	lines, err := r.ListLines(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		return err
	}
	requiredByStockItem := map[string]int64{}
	for _, l := range lines {
		sid, resErr := resolveStockItemIDDB(ctx, r.db, tenantID, regionID, l)
		if resErr != nil {
			return resErr
		}
		requiredByStockItem[sid] += l.Quantity
	}
	for stockItemID, required := range requiredByStockItem {
		var onHand int64
		if err := r.db.QueryRowContext(ctx, `
SELECT quantity FROM stock_items WHERE tenant_id = $1 AND region_id = $2 AND id = $3
`, tenantID, regionID, stockItemID).Scan(&onHand); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInsufficientStock
			}
			return err
		}
		var othersReserved int64
		if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(quantity), 0)
FROM stock_reservations
WHERE tenant_id = $1 AND region_id = $2 AND stock_item_id = $3
  AND checkout_id <> $4
  AND expires_at > NOW()
`, tenantID, regionID, stockItemID, checkoutID).Scan(&othersReserved); err != nil {
			return err
		}
		if onHand < othersReserved+required {
			return ErrInsufficientStock
		}
	}
	return nil
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

func (r *PostgresRepository) sessionStatus(ctx context.Context, tenantID, regionID, checkoutID string) (string, bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `
SELECT status
FROM checkout_sessions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, checkoutID, tenantID, regionID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return status, true, nil
}

func (r *PostgresRepository) getSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT `+checkoutSessionSelectColumns+`
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
		&out.GiftCardID,
		&out.GiftCardAppliedCents,
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
