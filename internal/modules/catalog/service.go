package catalog

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

func (s *Service) Save(ctx context.Context, product Product, idempotencyKey string) (Product, error) {
	if strings.TrimSpace(product.SKU) == "" || strings.TrimSpace(product.Name) == "" {
		return Product{}, sharederrors.BadRequest("sku and name are required")
	}
	if len(product.Currency) != 3 || strings.ToUpper(product.Currency) != product.Currency {
		return Product{}, sharederrors.BadRequest("currency must be ISO 4217 uppercase")
	}
	if product.PriceCents <= 0 {
		return Product{}, sharederrors.BadRequest("price_cents must be positive")
	}
	saved, err := s.repo.Upsert(ctx, product, idempotencyKey)
	if err != nil {
		return Product{}, sharederrors.Conflict("product conflicts with existing sku in tenant/region")
	}
	s.bus.Publish(ctx, events.EventProductUpdated, saved)
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, tenantID, regionID, sku, cursor, limit)
}
