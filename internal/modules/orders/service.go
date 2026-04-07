package orders

import (
	"context"
	"strings"
	"time"

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

func (s *Service) Create(ctx context.Context, order Order, idempotencyKey string) (Order, error) {
	if order.Status == "" {
		order.Status = "created"
	}
	if strings.TrimSpace(order.CustomerID) == "" || order.TotalCents <= 0 || len(order.Currency) != 3 {
		return Order{}, sharederrors.BadRequest("invalid order payload")
	}
	saved, err := s.repo.Insert(ctx, order, idempotencyKey)
	if err != nil {
		return Order{}, sharederrors.Internal("failed to persist order")
	}
	s.bus.Publish(ctx, events.EventOrderCreated, saved)
	return saved, nil
}

func (s *Service) UpdateStatus(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	if input.ExpectedUpdatedAt.IsZero() {
		return Order{}, sharederrors.BadRequest("expected_updated_at is required")
	}
	currentOrder, err := s.repo.GetByID(ctx, tenantID, input.ID)
	if err != nil {
		return Order{}, sharederrors.Internal("failed to load current order state")
	}
	if !isValidTransition(currentOrder.Status, input.Status) {
		return Order{}, sharederrors.BadRequest(ErrInvalidStatusTransition.Error())
	}
	saved, err := s.repo.UpdateStatus(ctx, tenantID, input)
	if err != nil {
		if err == ErrOptimisticLockFailed {
			return Order{}, sharederrors.Conflict(err.Error())
		}
		return Order{}, sharederrors.Internal("failed to update order status")
	}
	if saved.Status == "completed" {
		s.bus.Publish(ctx, events.EventOrderCompleted, saved)
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID string, cursor *time.Time, limit int32) ([]Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, tenantID, regionID, cursor, limit)
}

func isValidTransition(from, to string) bool {
	switch from {
	case "created":
		return to == "confirmed" || to == "cancelled" || to == "created"
	case "confirmed":
		return to == "completed" || to == "cancelled" || to == "confirmed"
	case "completed", "cancelled":
		return to == from
	default:
		return to == "created"
	}
}
