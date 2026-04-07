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

func (s *Service) SaveVariant(ctx context.Context, variant ProductVariant) (ProductVariant, error) {
	if strings.TrimSpace(variant.ProductID) == "" || strings.TrimSpace(variant.SKU) == "" || strings.TrimSpace(variant.Name) == "" {
		return ProductVariant{}, sharederrors.BadRequest("product_id, sku and name are required")
	}
	if len(variant.Currency) != 3 || strings.ToUpper(variant.Currency) != variant.Currency {
		return ProductVariant{}, sharederrors.BadRequest("currency must be ISO 4217 uppercase")
	}
	if variant.PriceCents <= 0 {
		return ProductVariant{}, sharederrors.BadRequest("price_cents must be positive")
	}
	unique, err := s.repo.IsSKUTenantRegionAvailable(ctx, variant.TenantID, variant.RegionID, variant.SKU, variant.ID)
	if err != nil {
		return ProductVariant{}, err
	}
	if !unique {
		return ProductVariant{}, sharederrors.Conflict("sku conflicts with existing product or variant in tenant/region")
	}
	saved, err := s.repo.UpsertVariant(ctx, variant)
	if err != nil {
		return ProductVariant{}, sharederrors.Conflict("variant conflicts with existing sku in tenant/region")
	}
	return saved, nil
}

func (s *Service) ListVariants(ctx context.Context, tenantID, regionID, productID string) ([]ProductVariant, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, sharederrors.BadRequest("product_id is required")
	}
	return s.repo.ListVariants(ctx, tenantID, regionID, productID)
}

func (s *Service) SaveCategory(ctx context.Context, category Category) (Category, error) {
	if strings.TrimSpace(category.Name) == "" || strings.TrimSpace(category.Slug) == "" {
		return Category{}, sharederrors.BadRequest("name and slug are required")
	}
	saved, err := s.repo.InsertCategory(ctx, category)
	if err != nil {
		return Category{}, sharederrors.Conflict("category conflicts with existing slug in tenant/region")
	}
	return saved, nil
}

func (s *Service) ListCategories(ctx context.Context, tenantID, regionID string) ([]Category, error) {
	return s.repo.ListCategories(ctx, tenantID, regionID)
}

func (s *Service) SaveCollection(ctx context.Context, collection Collection) (Collection, error) {
	if strings.TrimSpace(collection.Name) == "" || strings.TrimSpace(collection.Slug) == "" {
		return Collection{}, sharederrors.BadRequest("name and slug are required")
	}
	saved, err := s.repo.InsertCollection(ctx, collection)
	if err != nil {
		return Collection{}, sharederrors.Conflict("collection conflicts with existing slug in tenant/region")
	}
	return saved, nil
}

func (s *Service) ListCollections(ctx context.Context, tenantID, regionID string) ([]Collection, error) {
	return s.repo.ListCollections(ctx, tenantID, regionID)
}

func (s *Service) AssignProductToCollection(ctx context.Context, tenantID, regionID, collectionID, productID string) error {
	if strings.TrimSpace(collectionID) == "" || strings.TrimSpace(productID) == "" {
		return sharederrors.BadRequest("collection_id and product_id are required")
	}
	return s.repo.AssignProductToCollection(ctx, tenantID, regionID, collectionID, productID)
}

func (s *Service) SaveProductMedia(ctx context.Context, media ProductMedia) (ProductMedia, error) {
	if strings.TrimSpace(media.ProductID) == "" || strings.TrimSpace(media.URL) == "" || strings.TrimSpace(media.MediaType) == "" {
		return ProductMedia{}, sharederrors.BadRequest("product_id, url and media_type are required")
	}
	saved, err := s.repo.InsertProductMedia(ctx, media)
	if err != nil {
		return ProductMedia{}, err
	}
	return saved, nil
}

func (s *Service) ListProductMedia(ctx context.Context, tenantID, regionID, productID string) ([]ProductMedia, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, sharederrors.BadRequest("product_id is required")
	}
	return s.repo.ListProductMedia(ctx, tenantID, regionID, productID)
}
