package fulfillments

import (
	"context"
	"errors"
	"strings"

	"rewrite/internal/modules/orders"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

type Service struct {
	repo Repository
	bus  *events.Bus
}

func NewService(repo Repository, bus *events.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) Create(ctx context.Context, in Fulfillment, idempotencyKey string) (Fulfillment, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return Fulfillment{}, sharederrors.BadRequest("order_id is required")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return Fulfillment{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = "fulfilled"
	}
	if s.bus == nil {
		return Fulfillment{}, sharederrors.Internal("event bus is not configured")
	}
	res, err := s.repo.Create(ctx, in, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return Fulfillment{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrOrderNotFulfillable) {
			return Fulfillment{}, sharederrors.Conflict(err.Error())
		}
		if errors.Is(err, orders.ErrOptimisticLockFailed) {
			return Fulfillment{}, sharederrors.Conflict(err.Error())
		}
		return Fulfillment{}, sharederrors.Internal("failed to create fulfillment")
	}
	if !res.FromIdempotencyReplay && res.EmitOrderCompleted {
		s.bus.Publish(ctx, events.EventOrderCompleted, res.FinalOrder)
	}
	return res.Fulfillment, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error) {
	return s.repo.List(ctx, tenantID, regionID, orderID)
}
