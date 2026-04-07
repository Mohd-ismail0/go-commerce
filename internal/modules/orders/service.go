package orders

import (
	"context"

	"rewrite/internal/shared/events"
)

type Service struct {
	repo *Repository
	bus  *events.Bus
}

func NewService(repo *Repository, bus *events.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) Create(ctx context.Context, order Order) Order {
	if order.Status == "" {
		order.Status = "created"
	}
	saved := s.repo.Save(order)
	s.bus.Publish(ctx, events.EventOrderCreated, saved)
	return saved
}

func (s *Service) UpdateStatus(ctx context.Context, order Order) Order {
	saved := s.repo.Save(order)
	if saved.Status == "completed" {
		s.bus.Publish(ctx, events.EventOrderCompleted, saved)
	}
	return saved
}

func (s *Service) List(_ context.Context, tenantID string) []Order {
	return s.repo.List(tenantID)
}
