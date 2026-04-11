package checkout

import (
	"context"
	"errors"
	"strings"

	"rewrite/internal/modules/pricing"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/utils"
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

func (s *Service) CreateSession(ctx context.Context, in Session) (Session, error) {
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
	return s.repo.CreateSession(ctx, in)
}

func (s *Service) UpsertLine(ctx context.Context, tenantID, regionID string, in Line) (Line, error) {
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
	line, err := s.repo.UpsertLine(ctx, tenantID, regionID, in)
	if err != nil {
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

func (s *Service) UpdateSessionContext(ctx context.Context, tenantID, regionID, checkoutID string, in Session) (Session, error) {
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
	updated, err := s.repo.UpdateSessionContext(ctx, tenantID, regionID, checkoutID, in)
	if err != nil {
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
	return s.Recalculate(ctx, tenantID, regionID, checkoutID)
}

func (s *Service) Recalculate(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
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
	session, err := s.repo.Recalculate(ctx, tenantID, regionID, checkoutID, opts)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrSessionNotOpen) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, ErrShippingAddressCountryRequired) || errors.Is(err, ErrShippingMethodNotEligible) {
			return Session{}, sharederrors.Conflict(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to recalculate checkout totals")
	}
	return session, nil
}

func (s *Service) Complete(ctx context.Context, tenantID, regionID, checkoutID string) (CompleteResult, error) {
	recalculated, err := s.Recalculate(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		return CompleteResult{}, err
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
	orderID := utils.NewID("ord")
	saved, err := s.repo.Complete(ctx, tenantID, regionID, checkoutID, orderID)
	if err != nil {
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
		if errors.Is(err, ErrChannelListingMismatch) {
			return CompleteResult{}, sharederrors.Conflict(err.Error())
		}
		return CompleteResult{}, sharederrors.Internal("failed to complete checkout")
	}
	s.bus.Publish(ctx, events.EventOrderCreated, saved)
	return CompleteResult{OrderID: saved.ID}, nil
}
