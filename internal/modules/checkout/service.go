package checkout

import (
	"context"
	"errors"
	"strings"

	"rewrite/internal/modules/pricing"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

type Service struct {
	repo       Repository
	bus        *events.Bus
	calculator PricingCalculator
}

type PricingCalculator interface {
	Calculate(ctx context.Context, in pricing.CalculationInput) pricing.CalculationResult
}

func NewService(repo Repository, bus *events.Bus, calculator PricingCalculator) *Service {
	return &Service{repo: repo, bus: bus, calculator: calculator}
}

// GetSession returns the checkout session scoped by tenant and region.
// ValidateCheckout returns aggregated readiness problems (read-only; does not mutate totals).
func (s *Service) ValidateCheckout(ctx context.Context, tenantID, regionID, checkoutID string) (CheckoutValidationReport, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	if checkoutID == "" {
		return CheckoutValidationReport{}, sharederrors.BadRequest("checkout_id is required")
	}
	sess, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return CheckoutValidationReport{}, sharederrors.NotFound(err.Error())
		}
		return CheckoutValidationReport{}, sharederrors.Internal("failed to load checkout session")
	}
	report := CheckoutValidationReport{CheckoutID: checkoutID, Problems: []CheckoutProblem{}}
	if !strings.EqualFold(strings.TrimSpace(sess.Status), "open") {
		report.Problems = append(report.Problems, CheckoutProblem{
			Code: "session_not_open", Message: "checkout session is not open", Severity: "error",
		})
		return report, nil
	}
	lines, err := s.repo.ListLines(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		return CheckoutValidationReport{}, sharederrors.Internal("failed to load checkout lines")
	}
	if len(lines) == 0 {
		report.Problems = append(report.Problems, CheckoutProblem{
			Code: "empty_cart", Message: "checkout has no line items", Severity: "warning",
		})
	}
	if len(lines) > 0 {
		if strings.TrimSpace(sess.ShippingMethodID) == "" {
			report.Problems = append(report.Problems, CheckoutProblem{
				Code: "shipping_method_required", Message: "shipping_method_id is required before completion", Severity: "error",
			})
		}
		if strings.TrimSpace(sess.ShippingAddressCountry) == "" {
			report.Problems = append(report.Problems, CheckoutProblem{
				Code: "shipping_address_required", Message: "shipping_address_country is required before completion", Severity: "error",
			})
		}
		covered, err := s.repo.HasAuthorizedPaymentCoverage(ctx, tenantID, regionID, checkoutID, sess.TotalCents)
		if err != nil {
			return CheckoutValidationReport{}, sharederrors.Internal("failed to validate payment coverage")
		}
		if !covered {
			report.Problems = append(report.Problems, CheckoutProblem{
				Code: "payment_coverage_required", Message: "authorized payment coverage is required before completion", Severity: "error",
			})
		}
		if strings.TrimSpace(sess.GiftCardID) != "" {
			if gerr := s.repo.ValidateGiftCardForSession(ctx, tenantID, regionID, sess); gerr != nil {
				switch {
				case errors.Is(gerr, ErrGiftCardNotFound):
					report.Problems = append(report.Problems, CheckoutProblem{Code: "gift_card_not_found", Message: gerr.Error(), Severity: "error"})
				case errors.Is(gerr, ErrGiftCardInactive):
					report.Problems = append(report.Problems, CheckoutProblem{Code: "gift_card_inactive", Message: gerr.Error(), Severity: "error"})
				case errors.Is(gerr, ErrGiftCardExpired):
					report.Problems = append(report.Problems, CheckoutProblem{Code: "gift_card_expired", Message: gerr.Error(), Severity: "error"})
				case errors.Is(gerr, ErrGiftCardCurrencyMismatch):
					report.Problems = append(report.Problems, CheckoutProblem{Code: "gift_card_currency_mismatch", Message: gerr.Error(), Severity: "error"})
				case errors.Is(gerr, ErrGiftCardDepleted):
					report.Problems = append(report.Problems, CheckoutProblem{Code: "gift_card_depleted", Message: gerr.Error(), Severity: "error"})
				default:
					return CheckoutValidationReport{}, sharederrors.Internal("failed to validate gift card")
				}
			}
		}
		if ch := strings.TrimSpace(sess.ChannelID); ch != "" {
			active, err := s.repo.ChannelIsActive(ctx, tenantID, regionID, ch)
			if err != nil {
				return CheckoutValidationReport{}, sharederrors.Internal("failed to validate channel")
			}
			if !active {
				report.Problems = append(report.Problems, CheckoutProblem{
					Code: "channel_inactive", Message: "checkout channel is not active", Severity: "error",
				})
			} else {
				for _, line := range lines {
					if strings.TrimSpace(line.VariantID) != "" {
						priceCents, currency, isPublished, found, getErr := s.repo.GetVariantChannelListing(ctx, tenantID, regionID, ch, line.VariantID)
						if getErr != nil {
							return CheckoutValidationReport{}, sharederrors.Internal("failed to validate channel variant listing")
						}
						if !found || !isPublished || line.UnitPriceCents != priceCents || !strings.EqualFold(strings.TrimSpace(line.Currency), strings.TrimSpace(currency)) {
							report.Problems = append(report.Problems, CheckoutProblem{
								Code: "variant_listing_mismatch", Message: "a variant line does not match channel listing", Severity: "error",
							})
						}
						continue
					}
					if strings.TrimSpace(line.ProductID) != "" {
						isPublished, found, getErr := s.repo.GetProductChannelListing(ctx, tenantID, regionID, ch, line.ProductID)
						if getErr != nil {
							return CheckoutValidationReport{}, sharederrors.Internal("failed to validate channel product listing")
						}
						if !found || !isPublished {
							report.Problems = append(report.Problems, CheckoutProblem{
								Code: "product_listing_mismatch", Message: "a product line is not published in checkout channel", Severity: "error",
							})
						}
					}
				}
			}
		}
		if err := s.repo.ValidateCheckoutStock(ctx, tenantID, regionID, checkoutID); err != nil {
			if errors.Is(err, ErrInsufficientStock) {
				report.Problems = append(report.Problems, CheckoutProblem{
					Code: "insufficient_stock", Message: err.Error(), Severity: "error",
				})
			} else {
				return CheckoutValidationReport{}, sharederrors.Internal("failed to validate stock")
			}
		}
	}
	return report, nil
}

func (s *Service) GetSession(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	if checkoutID == "" {
		return Session{}, sharederrors.BadRequest("checkout_id is required")
	}
	sess, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to load checkout session")
	}
	return sess, nil
}

// ListLines returns checkout lines after verifying the session exists for tenant/region (404 when missing).
func (s *Service) ListLines(ctx context.Context, tenantID, regionID, checkoutID string) ([]Line, error) {
	checkoutID = strings.TrimSpace(checkoutID)
	if checkoutID == "" {
		return nil, sharederrors.BadRequest("checkout_id is required")
	}
	if _, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, sharederrors.NotFound(err.Error())
		}
		return nil, sharederrors.Internal("failed to load checkout session")
	}
	lines, err := s.repo.ListLines(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list checkout lines")
	}
	if lines == nil {
		return []Line{}, nil
	}
	return lines, nil
}

func (s *Service) CreateSession(ctx context.Context, in Session, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(in.CustomerID) == "" || len(strings.TrimSpace(in.Currency)) != 3 {
		return Session{}, sharederrors.BadRequest("invalid checkout payload")
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.ShippingMethodID = strings.TrimSpace(in.ShippingMethodID)
	in.ShippingAddressCountry = strings.ToUpper(strings.TrimSpace(in.ShippingAddressCountry))
	in.ShippingAddressPostalCode = strings.TrimSpace(in.ShippingAddressPostalCode)
	in.BillingAddressCountry = strings.ToUpper(strings.TrimSpace(in.BillingAddressCountry))
	in.BillingAddressPostalCode = strings.TrimSpace(in.BillingAddressPostalCode)
	if strings.TrimSpace(in.ChannelID) != "" {
		active, err := s.repo.ChannelIsActive(ctx, in.TenantID, in.RegionID, strings.TrimSpace(in.ChannelID))
		if err != nil {
			return Session{}, sharederrors.Internal("failed to validate checkout channel")
		}
		if !active {
			return Session{}, sharederrors.BadRequest("channel_id must reference an active channel")
		}
	}
	if in.Status == "" {
		in.Status = "open"
	}
	saved, err := s.repo.CreateSession(ctx, in, key)
	if err != nil {
		if errors.Is(err, ErrIdempotencyKeyRequired) {
			return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutIdempotencyOrphan) {
			return Session{}, sharederrors.Internal("checkout idempotency record is inconsistent")
		}
		return Session{}, sharederrors.Internal("failed to create checkout session")
	}
	return saved, nil
}

func (s *Service) UpsertLine(ctx context.Context, tenantID, regionID string, in Line, idempotencyKey string) (Line, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Line{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(in.CheckoutID) == "" || strings.TrimSpace(in.ID) == "" {
		return Line{}, sharederrors.BadRequest("checkout_id and id are required")
	}
	if in.Quantity <= 0 || in.UnitPriceCents <= 0 || len(strings.TrimSpace(in.Currency)) != 3 {
		return Line{}, sharederrors.BadRequest("invalid checkout line")
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.VariantID = strings.TrimSpace(in.VariantID)
	if in.ProductID == "" && in.VariantID == "" {
		return Line{}, sharederrors.BadRequest("either product_id or variant_id is required")
	}
	session, err := s.repo.GetSession(ctx, tenantID, regionID, in.CheckoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Line{}, sharederrors.NotFound(err.Error())
		}
		return Line{}, sharederrors.Internal("failed to load checkout session")
	}
	if !strings.EqualFold(strings.TrimSpace(session.Status), "open") {
		return Line{}, sharederrors.Conflict("checkout session is not open")
	}
	if !strings.EqualFold(strings.TrimSpace(session.Currency), in.Currency) {
		return Line{}, sharederrors.BadRequest("checkout line currency must match session currency")
	}
	if in.VariantID != "" {
		variantProductID, found, vErr := s.repo.GetVariantProductID(ctx, tenantID, regionID, in.VariantID)
		if vErr != nil {
			return Line{}, sharederrors.Internal("failed to validate variant identity")
		}
		if !found {
			return Line{}, sharederrors.NotFound("variant not found")
		}
		if in.ProductID != "" && in.ProductID != variantProductID {
			return Line{}, sharederrors.BadRequest("product_id does not match variant")
		}
		if in.ProductID == "" {
			in.ProductID = variantProductID
		}
	}
	if in.VariantID != "" && strings.TrimSpace(session.ChannelID) != "" {
		priceCents, currency, isPublished, found, getErr := s.repo.GetVariantChannelListing(ctx, tenantID, regionID, session.ChannelID, in.VariantID)
		if getErr != nil {
			return Line{}, sharederrors.Internal("failed to validate channel variant listing")
		}
		if !found || !isPublished {
			return Line{}, sharederrors.Conflict("variant is not published in checkout channel")
		}
		if in.UnitPriceCents != priceCents || !strings.EqualFold(strings.TrimSpace(in.Currency), strings.TrimSpace(currency)) {
			return Line{}, sharederrors.BadRequest("checkout line price/currency must match channel variant listing")
		}
	}
	if in.ProductID != "" && in.VariantID == "" && strings.TrimSpace(session.ChannelID) != "" {
		isPublished, found, getErr := s.repo.GetProductChannelListing(ctx, tenantID, regionID, session.ChannelID, in.ProductID)
		if getErr != nil {
			return Line{}, sharederrors.Internal("failed to validate channel product listing")
		}
		if !found || !isPublished {
			return Line{}, sharederrors.Conflict("product is not published in checkout channel")
		}
	}
	line, err := s.repo.UpsertLine(ctx, tenantID, regionID, in, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutLineUpsertIdempotencyKeyRequired) {
			return Line{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutLineUpsertIdempotencyOrphan) || errors.Is(err, ErrCheckoutLineUpsertIdempotencyMismatch) {
			return Line{}, sharederrors.Internal("checkout line idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Line{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Line{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrInsufficientStock) {
			return Line{}, sharederrors.Conflict(err.Error())
		}
		return Line{}, sharederrors.Internal("failed to save checkout line")
	}
	return line, nil
}

func (s *Service) UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(checkoutID) == "" {
		return Session{}, sharederrors.BadRequest("checkout_id is required")
	}
	current, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to load checkout session")
	}
	if !strings.EqualFold(strings.TrimSpace(current.Status), "open") {
		return Session{}, sharederrors.Conflict("checkout session is not open")
	}
	targetChannelID := strings.TrimSpace(in.ChannelID)
	in.ShippingMethodID = strings.TrimSpace(in.ShippingMethodID)
	in.ShippingAddressCountry = strings.ToUpper(strings.TrimSpace(in.ShippingAddressCountry))
	in.ShippingAddressPostalCode = strings.TrimSpace(in.ShippingAddressPostalCode)
	in.BillingAddressCountry = strings.ToUpper(strings.TrimSpace(in.BillingAddressCountry))
	in.BillingAddressPostalCode = strings.TrimSpace(in.BillingAddressPostalCode)
	channelIsChanging := targetChannelID != strings.TrimSpace(current.ChannelID)
	if targetChannelID != "" {
		active, err := s.repo.ChannelIsActive(ctx, tenantID, regionID, targetChannelID)
		if err != nil {
			return Session{}, sharederrors.Internal("failed to validate checkout channel")
		}
		if !active {
			return Session{}, sharederrors.BadRequest("channel_id must reference an active channel")
		}
	}
	if channelIsChanging {
		lines, err := s.repo.ListLines(ctx, tenantID, regionID, checkoutID)
		if err != nil {
			return Session{}, sharederrors.Internal("failed to validate existing checkout lines")
		}
		if len(lines) > 0 {
			if targetChannelID == "" {
				return Session{}, sharederrors.Conflict("cannot unset channel_id while checkout has lines")
			}
			for _, line := range lines {
				if strings.TrimSpace(line.VariantID) != "" {
					priceCents, currency, isPublished, found, getErr := s.repo.GetVariantChannelListing(ctx, tenantID, regionID, targetChannelID, line.VariantID)
					if getErr != nil {
						return Session{}, sharederrors.Internal("failed to validate channel variant listing")
					}
					if !found || !isPublished || line.UnitPriceCents != priceCents || !strings.EqualFold(strings.TrimSpace(line.Currency), strings.TrimSpace(currency)) {
						return Session{}, sharederrors.Conflict("cannot change channel_id: existing variant lines are incompatible")
					}
					continue
				}
				if strings.TrimSpace(line.ProductID) != "" {
					isPublished, found, getErr := s.repo.GetProductChannelListing(ctx, tenantID, regionID, targetChannelID, line.ProductID)
					if getErr != nil {
						return Session{}, sharederrors.Internal("failed to validate channel product listing")
					}
					if !found || !isPublished {
						return Session{}, sharederrors.Conflict("cannot change channel_id: existing product lines are incompatible")
					}
				}
			}
		}
	}
	updated, err := s.repo.UpdateSessionContext(ctx, tenantID, regionID, checkoutID, in, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutPatchSessionIdempotencyKeyRequired) {
			return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutPatchSessionIdempotencyOrphan) || errors.Is(err, ErrCheckoutPatchSessionIdempotencyMismatch) {
			return Session{}, sharederrors.Internal("checkout patch session idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to update checkout session context")
	}
	if s.calculator == nil {
		return updated, nil
	}
	return s.Recalculate(ctx, tenantID, regionID, checkoutID, "")
}

// ApplyCustomerAddresses copies country and postal code from saved customer_addresses rows onto the open checkout (same customer_id), under one DB transaction with a session row lock.
func (s *Service) ApplyCustomerAddresses(ctx context.Context, tenantID, regionID, checkoutID, shippingAddressID, billingAddressID, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(checkoutID) == "" {
		return Session{}, sharederrors.BadRequest("checkout_id is required")
	}
	if strings.TrimSpace(shippingAddressID) == "" && strings.TrimSpace(billingAddressID) == "" {
		return Session{}, sharederrors.BadRequest("at least one of shipping_address_id or billing_address_id is required")
	}
	session, err := s.repo.ApplyCustomerAddressesToCheckout(ctx, tenantID, regionID, checkoutID, shippingAddressID, billingAddressID, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutApplyAddressesIdempotencyKeyRequired) {
			return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutApplyAddressesIdempotencyOrphan) || errors.Is(err, ErrCheckoutApplyAddressesIdempotencyMismatch) {
			return Session{}, sharederrors.Internal("checkout apply-addresses idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrCustomerAddressNotApplicable) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to apply customer addresses to checkout")
	}
	if s.calculator == nil {
		return session, nil
	}
	return s.Recalculate(ctx, tenantID, regionID, checkoutID, "")
}

// ApplyGiftCard attaches a gift card to an open checkout (at most one open checkout may hold a given card). Recalculate runs when a pricing calculator is configured.
func (s *Service) ApplyGiftCard(ctx context.Context, tenantID, regionID, checkoutID, code, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(checkoutID) == "" {
		return Session{}, sharederrors.BadRequest("checkout_id is required")
	}
	session, err := s.repo.ApplyGiftCardToCheckout(ctx, tenantID, regionID, checkoutID, code, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutGiftCardApplyIdempotencyKeyRequired) {
			return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutGiftCardApplyIdempotencyOrphan) || errors.Is(err, ErrCheckoutGiftCardApplyIdempotencyMismatch) {
			return Session{}, sharederrors.Internal("checkout gift card apply idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrGiftCardNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrGiftCardInactive) || errors.Is(err, ErrGiftCardExpired) || errors.Is(err, ErrGiftCardCurrencyMismatch) || errors.Is(err, ErrGiftCardInUse) || errors.Is(err, ErrGiftCardDepleted) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to apply gift card to checkout")
	}
	if s.calculator == nil {
		return session, nil
	}
	return s.Recalculate(ctx, tenantID, regionID, checkoutID, "")
}

// RemoveGiftCard clears a gift card from an open checkout. Recalculate runs when a pricing calculator is configured.
func (s *Service) RemoveGiftCard(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(checkoutID) == "" {
		return Session{}, sharederrors.BadRequest("checkout_id is required")
	}
	session, err := s.repo.RemoveGiftCardFromCheckout(ctx, tenantID, regionID, checkoutID, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutGiftCardRemoveIdempotencyKeyRequired) {
			return Session{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutGiftCardRemoveIdempotencyOrphan) || errors.Is(err, ErrCheckoutGiftCardRemoveIdempotencyMismatch) {
			return Session{}, sharederrors.Internal("checkout gift card remove idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to remove gift card from checkout")
	}
	if s.calculator == nil {
		return session, nil
	}
	return s.Recalculate(ctx, tenantID, regionID, checkoutID, "")
}

// Recalculate refreshes subtotal from lines, then shipping and tax/total when a pricing calculator is configured.
// Pass an empty idempotencyKey for internal orchestration (e.g. complete checkout, context updates); the HTTP handler always supplies a key.
func (s *Service) Recalculate(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	var opts *RecalculateOptions
	if s.calculator != nil {
		opts = &RecalculateOptions{
			ComputePricing: func(ctx context.Context, session Session, baseAmountCents int64) (int64, int64, error) {
				res := s.calculator.Calculate(ctx, pricing.CalculationInput{
					TenantID:        tenantID,
					RegionID:        regionID,
					Currency:        session.Currency,
					BaseAmountCents: baseAmountCents,
					VoucherCode:     strings.TrimSpace(session.VoucherCode),
					PromotionID:     strings.TrimSpace(session.PromotionID),
					TaxClassID:      strings.TrimSpace(session.TaxClassID),
					CountryCode:     strings.TrimSpace(session.CountryCode),
				})
				return res.TaxCents, res.TotalCents, nil
			},
		}
	}
	session, err := s.repo.Recalculate(ctx, tenantID, regionID, checkoutID, opts, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutRecalculateIdempotencyOrphan) || errors.Is(err, ErrCheckoutRecalculateIdempotencyMismatch) {
			return Session{}, sharederrors.Internal("checkout recalculate idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrShippingAddressCountryRequired) || errors.Is(err, ErrShippingMethodNotEligible) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrGiftCardNotFound) || errors.Is(err, ErrGiftCardInactive) || errors.Is(err, ErrGiftCardExpired) || errors.Is(err, ErrGiftCardCurrencyMismatch) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to recalculate checkout totals")
	}
	return session, nil
}

func (s *Service) Complete(ctx context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (CompleteResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return CompleteResult{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	checkoutID = strings.TrimSpace(checkoutID)

	sess, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return CompleteResult{}, sharederrors.NotFound(err.Error())
		}
		return CompleteResult{}, sharederrors.Internal("failed to load checkout session")
	}
	if strings.EqualFold(strings.TrimSpace(sess.Status), "open") {
		if _, err := s.Recalculate(ctx, tenantID, regionID, checkoutID, ""); err != nil {
			return CompleteResult{}, err
		}
	}

	recalculated, err := s.repo.GetSession(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return CompleteResult{}, sharederrors.NotFound(err.Error())
		}
		return CompleteResult{}, sharederrors.Internal("failed to load checkout session")
	}
	lines, err := s.repo.ListLines(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		return CompleteResult{}, sharederrors.Internal("failed to validate checkout lines")
	}
	if len(lines) > 0 {
		if strings.TrimSpace(recalculated.ShippingMethodID) == "" {
			return CompleteResult{}, sharederrors.Conflict("shipping_method_id is required before checkout completion")
		}
		if strings.TrimSpace(recalculated.ShippingAddressCountry) == "" {
			return CompleteResult{}, sharederrors.Conflict("shipping_address_country is required before checkout completion")
		}
	}
	covered, err := s.repo.HasAuthorizedPaymentCoverage(ctx, tenantID, regionID, checkoutID, recalculated.TotalCents)
	if err != nil {
		return CompleteResult{}, sharederrors.Internal("failed to validate payment coverage")
	}
	if !covered {
		return CompleteResult{}, sharederrors.Conflict("authorized payment coverage is required before checkout completion")
	}
	outcome, err := s.repo.Complete(ctx, tenantID, regionID, checkoutID, key)
	if err != nil {
		if errors.Is(err, ErrCheckoutCompleteIdempotencyKeyRequired) {
			return CompleteResult{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCheckoutCompleteIdempotencyOrphan) {
			return CompleteResult{}, sharederrors.Internal("checkout completion idempotency record is inconsistent")
		}
		if errors.Is(err, ErrSessionNotFound) {
			return CompleteResult{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) || errors.Is(err, ErrCheckoutEmpty) {
			return CompleteResult{}, sharederrors.BadRequest(err.Error())
		}
		if errors.Is(err, ErrInsufficientStock) {
			return CompleteResult{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrVoucherUnavailable) {
			return CompleteResult{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrGiftCardDepleted) {
			return CompleteResult{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrChannelListingMismatch) {
			return CompleteResult{}, sharederrors.Conflict(err.Error())
		}
		return CompleteResult{}, sharederrors.Internal("failed to complete checkout")
	}
	if !outcome.FromIdempotencyReplay {
		s.bus.Publish(ctx, events.EventOrderCreated, outcome.Payload)
	}
	return CompleteResult{OrderID: outcome.Payload.ID}, nil
}
