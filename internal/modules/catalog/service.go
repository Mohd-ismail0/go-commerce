package catalog

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

func (s *Service) Save(ctx context.Context, product Product) (Product, error) {
	saved, err := s.repo.Upsert(ctx, product)
	if err != nil {
		return Product{}, err
	}
	s.bus.Publish(ctx, events.EventProductUpdated, saved)
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, sku string) ([]Product, error) {
	return s.repo.List(ctx, tenantID, regionID, sku)
}
