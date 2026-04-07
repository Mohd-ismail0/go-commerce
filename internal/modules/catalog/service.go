package catalog

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

func (s *Service) Save(ctx context.Context, product Product) Product {
	saved := s.repo.Upsert(product)
	s.bus.Publish(ctx, events.EventProductUpdated, saved)
	return saved
}

func (s *Service) List(_ context.Context, tenantID string) []Product {
	return s.repo.List(tenantID)
}
