package inventory

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

func (s *Service) Save(ctx context.Context, item StockItem) StockItem {
	saved := s.repo.Save(item)
	s.bus.Publish(ctx, events.EventInventoryChange, saved)
	return saved
}

func (s *Service) List(_ context.Context, tenantID string) []StockItem {
	return s.repo.List(tenantID)
}
