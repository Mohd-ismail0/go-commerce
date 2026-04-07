package orders

import (
	"context"

	"rewrite/internal/shared/events"
)

type Service struct {
	repo Repository
	bus  *events.Bus
}

func NewService(repo Repository, bus *events.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) Create(ctx context.Context, order Order) (Order, error) {
	if order.Status == "" {
		order.Status = "created"
	}
	saved, err := s.repo.Insert(ctx, order)
	if err != nil {
		return Order{}, err
	}
	s.bus.Publish(ctx, events.EventOrderCreated, saved)
	return saved, nil
}

func (s *Service) UpdateStatus(ctx context.Context, tenantID, orderID, status string) (Order, error) {
	if !isValidTransition(status) {
		return Order{}, ErrInvalidStatusTransition
	}
	saved, err := s.repo.UpdateStatus(ctx, tenantID, orderID, status)
	if err != nil {
		return Order{}, err
	}
	if saved.Status == "completed" {
		s.bus.Publish(ctx, events.EventOrderCompleted, saved)
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID string) ([]Order, error) {
	return s.repo.List(ctx, tenantID, regionID)
}

func isValidTransition(status string) bool {
	switch status {
	case "created", "confirmed", "completed", "cancelled":
		return true
	default:
		return false
	}
}
