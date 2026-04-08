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
	line, err := s.repo.UpsertLine(ctx, tenantID, regionID, in)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Line{}, sharederrors.NotFound(err.Error())
		}
		return Line{}, sharederrors.Internal("failed to save checkout line")
	}
	return line, nil
}

func (s *Service) Recalculate(ctx context.Context, tenantID, regionID, checkoutID string) (Session, error) {
	session, err := s.repo.Recalculate(ctx, tenantID, regionID, checkoutID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return Session{}, sharederrors.NotFound(err.Error())
		}
		return Session{}, sharederrors.Internal("failed to recalculate checkout totals")
	}
	if s.calculator == nil {
		return session, nil
	}
	calculated := s.calculator.Calculate(ctx, pricing.CalculationInput{
		TenantID:        tenantID,
		RegionID:        regionID,
		Currency:        session.Currency,
		BaseAmountCents: session.SubtotalCents + session.ShippingCents,
		VoucherCode:     strings.TrimSpace(session.VoucherCode),
		PromotionID:     strings.TrimSpace(session.PromotionID),
		TaxClassID:      strings.TrimSpace(session.TaxClassID),
		CountryCode:     strings.TrimSpace(session.CountryCode),
	})
	updated, updateErr := s.repo.UpdatePricing(ctx, tenantID, regionID, checkoutID, calculated.TaxCents, calculated.TotalCents)
	if updateErr != nil {
		return Session{}, sharederrors.Internal("failed to apply checkout pricing")
	}
	return updated, nil
}

func (s *Service) Complete(ctx context.Context, tenantID, regionID, checkoutID string) (CompleteResult, error) {
	if _, err := s.Recalculate(ctx, tenantID, regionID, checkoutID); err != nil {
		return CompleteResult{}, err
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
		return CompleteResult{}, sharederrors.Internal("failed to complete checkout")
	}
	s.bus.Publish(ctx, events.EventOrderCreated, saved)
	return CompleteResult{OrderID: saved.ID}, nil
}
