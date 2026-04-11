package inventory

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/utils"
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

func (s *Service) ListWarehouses(ctx context.Context, tenantID, regionID string) ([]Warehouse, error) {
	return s.repo.ListWarehouses(ctx, tenantID, regionID)
}

func (s *Service) SaveWarehouse(ctx context.Context, w Warehouse) (Warehouse, error) {
	if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Code) == "" {
		return Warehouse{}, sharederrors.BadRequest("warehouse name and code are required")
	}
	if w.ID == "" {
		w.ID = utils.NewID("wh")
	}
	saved, err := s.repo.SaveWarehouse(ctx, w)
	if err != nil {
		return Warehouse{}, sharederrors.Internal("failed to save warehouse")
	}
	s.bus.Publish(ctx, events.EventInventoryChange, saved)
	return saved, nil
}
