package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

func isFKViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

type Service struct {
	repo Repository
	bus  *events.Bus
}

func NewService(repo Repository, bus *events.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) Save(ctx context.Context, product Product, idempotencyKey string) (Product, error) {
	product.AttributeValues = nil
	if strings.TrimSpace(product.SKU) == "" || strings.TrimSpace(product.Name) == "" {
		return Product{}, sharederrors.BadRequest("sku and name are required")
	}
	if len(product.Currency) != 3 || strings.ToUpper(product.Currency) != product.Currency {
		return Product{}, sharederrors.BadRequest("currency must be ISO 4217 uppercase")
	}
	if product.PriceCents <= 0 {
		return Product{}, sharederrors.BadRequest("price_cents must be positive")
	}
	if strings.TrimSpace(product.Slug) != "" {
		unique, err := s.repo.IsProductSlugAvailable(ctx, product.TenantID, product.RegionID, strings.TrimSpace(product.Slug), product.ID)
		if err != nil {
			return Product{}, sharederrors.Internal("failed to validate product slug")
		}
		if !unique {
			return Product{}, sharederrors.Conflict("product conflicts with existing slug in tenant/region")
		}
	}
	if strings.TrimSpace(product.ProductTypeID) != "" {
		if _, err := s.repo.GetProductType(ctx, product.TenantID, product.RegionID, strings.TrimSpace(product.ProductTypeID)); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Product{}, sharederrors.BadRequest("product_type_id not found")
			}
			return Product{}, err
		}
	}
	saved, err := s.repo.Upsert(ctx, product, idempotencyKey)
	if err != nil {
		if isFKViolation(err) {
			return Product{}, sharederrors.BadRequest("invalid product_type_id")
		}
		return Product{}, sharederrors.Conflict("product conflicts with existing sku in tenant/region")
	}
	s.bus.Publish(ctx, events.EventProductUpdated, saved)
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, sku, languageCode, channelID string, onlyPublished bool, cursor *time.Time, limit int32, expandAttributeValues bool) ([]Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []Product
	var err error
	if strings.TrimSpace(channelID) != "" {
		items, err = s.repo.ListProductsByChannel(ctx, tenantID, regionID, strings.TrimSpace(channelID), sku, onlyPublished, cursor, limit)
	} else {
		items, err = s.repo.List(ctx, tenantID, regionID, sku, cursor, limit)
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(languageCode) == "" || len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	translations, err := s.repo.ListProductTranslations(ctx, tenantID, regionID, ids, strings.TrimSpace(languageCode))
	if err != nil {
		return nil, err
	}
	for i := range items {
		fields, ok := translations[items[i].ID]
		if !ok {
			continue
		}
		if v := strings.TrimSpace(fields["name"]); v != "" {
			items[i].Name = v
		}
		if v := strings.TrimSpace(fields["description"]); v != "" {
			items[i].Description = v
		}
		if v := strings.TrimSpace(fields["seo_title"]); v != "" {
			items[i].SEOTitle = v
		}
		if v := strings.TrimSpace(fields["seo_description"]); v != "" {
			items[i].SEODescription = v
		}
	}
	if expandAttributeValues && len(items) > 0 {
		ids := make([]string, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		byProduct, err := s.repo.ListProductAttributeValuesForProducts(ctx, ids)
		if err != nil {
			return nil, err
		}
		for i := range items {
			if v, ok := byProduct[items[i].ID]; ok && len(v) > 0 {
				items[i].AttributeValues = v
			}
		}
	}
	return items, nil
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

func (s *Service) ListVariants(ctx context.Context, tenantID, regionID, productID, channelID string, onlyPublished bool) ([]ProductVariant, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, sharederrors.BadRequest("product_id is required")
	}
	if strings.TrimSpace(channelID) != "" {
		return s.repo.ListVariantsByChannel(ctx, tenantID, regionID, productID, strings.TrimSpace(channelID), onlyPublished)
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

func (s *Service) ListCategories(ctx context.Context, tenantID, regionID, languageCode string) ([]Category, error) {
	items, err := s.repo.ListCategories(ctx, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(languageCode) == "" || len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	translations, err := s.repo.ListCategoryTranslations(ctx, tenantID, regionID, ids, strings.TrimSpace(languageCode))
	if err != nil {
		return nil, err
	}
	for i := range items {
		if fields, ok := translations[items[i].ID]; ok {
			if v := strings.TrimSpace(fields["name"]); v != "" {
				items[i].Name = v
			}
		}
	}
	return items, nil
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

func (s *Service) ListCollections(ctx context.Context, tenantID, regionID, languageCode string) ([]Collection, error) {
	items, err := s.repo.ListCollections(ctx, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(languageCode) == "" || len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	translations, err := s.repo.ListCollectionTranslations(ctx, tenantID, regionID, ids, strings.TrimSpace(languageCode))
	if err != nil {
		return nil, err
	}
	for i := range items {
		if fields, ok := translations[items[i].ID]; ok {
			if v := strings.TrimSpace(fields["name"]); v != "" {
				items[i].Name = v
			}
		}
	}
	return items, nil
}

func (s *Service) AssignProductToCollection(ctx context.Context, tenantID, regionID, collectionID, productID string) error {
	if strings.TrimSpace(collectionID) == "" || strings.TrimSpace(productID) == "" {
		return sharederrors.BadRequest("collection_id and product_id are required")
	}
	err := s.repo.AssignProductToCollection(ctx, tenantID, regionID, collectionID, productID)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAssignEntityNotFound) {
		return sharederrors.NotFound(err.Error())
	}
	if errors.Is(err, ErrCollectionProductAlreadyAssigned) {
		return sharederrors.Conflict(err.Error())
	}
	return sharederrors.Internal("failed to assign product to collection")
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

func (s *Service) SaveProductType(ctx context.Context, pt ProductType) (ProductType, error) {
	if strings.TrimSpace(pt.Name) == "" || strings.TrimSpace(pt.Slug) == "" {
		return ProductType{}, sharederrors.BadRequest("name and slug are required")
	}
	saved, err := s.repo.InsertProductType(ctx, pt)
	if err != nil {
		return ProductType{}, sharederrors.Conflict("product type conflicts with existing slug in tenant/region")
	}
	return saved, nil
}

func (s *Service) ListProductTypes(ctx context.Context, tenantID, regionID string) ([]ProductType, error) {
	return s.repo.ListProductTypes(ctx, tenantID, regionID)
}

func (s *Service) GetProductTypeDetail(ctx context.Context, tenantID, regionID, productTypeID string) (ProductTypeDetail, error) {
	if strings.TrimSpace(productTypeID) == "" {
		return ProductTypeDetail{}, sharederrors.BadRequest("product_type_id is required")
	}
	pt, err := s.repo.GetProductType(ctx, tenantID, regionID, productTypeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProductTypeDetail{}, sharederrors.NotFound("product type not found")
		}
		return ProductTypeDetail{}, err
	}
	attrs, err := s.repo.ListProductTypeAttributeDefs(ctx, tenantID, regionID, productTypeID)
	if err != nil {
		return ProductTypeDetail{}, err
	}
	return ProductTypeDetail{ProductType: pt, Attributes: attrs}, nil
}

func (s *Service) SaveCatalogAttribute(ctx context.Context, a CatalogAttribute) (CatalogAttribute, error) {
	if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Slug) == "" {
		return CatalogAttribute{}, sharederrors.BadRequest("name and slug are required")
	}
	if err := validateCatalogAttributeInput(a.InputType, a.AllowedValues); err != nil {
		return CatalogAttribute{}, err
	}
	saved, err := s.repo.InsertCatalogAttribute(ctx, a)
	if err != nil {
		return CatalogAttribute{}, sharederrors.Conflict("attribute conflicts with existing slug in tenant/region")
	}
	return saved, nil
}

func (s *Service) ListCatalogAttributes(ctx context.Context, tenantID, regionID string) ([]CatalogAttribute, error) {
	return s.repo.ListCatalogAttributes(ctx, tenantID, regionID)
}

func (s *Service) LinkAttributeToProductType(ctx context.Context, tenantID, regionID, productTypeID string, in LinkAttributeToTypeInput) error {
	if strings.TrimSpace(productTypeID) == "" || strings.TrimSpace(in.AttributeID) == "" {
		return sharederrors.BadRequest("product_type_id and attribute_id are required")
	}
	pt, err := s.repo.GetProductType(ctx, tenantID, regionID, productTypeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("product type not found")
		}
		return err
	}
	attr, err := s.repo.GetCatalogAttribute(ctx, tenantID, regionID, in.AttributeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("attribute not found")
		}
		return err
	}
	if pt.RegionID != attr.RegionID || pt.TenantID != attr.TenantID {
		return sharederrors.BadRequest("attribute and product type must belong to the same tenant and region")
	}
	if err := s.repo.LinkAttributeToProductType(ctx, productTypeID, in); err != nil {
		if isFKViolation(err) {
			return sharederrors.BadRequest("invalid product_type_id or attribute_id")
		}
		return err
	}
	return nil
}

func (s *Service) UnlinkAttributeFromProductType(ctx context.Context, tenantID, regionID, productTypeID, attributeID string) error {
	if strings.TrimSpace(productTypeID) == "" || strings.TrimSpace(attributeID) == "" {
		return sharederrors.BadRequest("product_type_id and attribute_id are required")
	}
	if err := s.repo.UnlinkAttributeFromProductType(ctx, tenantID, regionID, productTypeID, attributeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("product type not found")
		}
		return err
	}
	return nil
}

func (s *Service) ListProductAttributeValues(ctx context.Context, tenantID, regionID, productID string) ([]AttributeValuePair, error) {
	if err := s.assertProductInRegion(ctx, tenantID, regionID, productID); err != nil {
		return nil, err
	}
	return s.repo.ListProductAttributeValues(ctx, productID)
}

func (s *Service) SetProductAttributeValues(ctx context.Context, tenantID, regionID, productID string, pairs []AttributeValuePair) error {
	if err := s.assertProductInRegion(ctx, tenantID, regionID, productID); err != nil {
		return err
	}
	_, ptypeID, hasType, err := s.repo.GetProductRegionAndType(ctx, tenantID, productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("product not found")
		}
		return err
	}
	if !hasType {
		return sharederrors.BadRequest("product must have product_type_id to set attribute values")
	}
	normalized := make([]AttributeValuePair, 0, len(pairs))
	for _, pair := range pairs {
		if strings.TrimSpace(pair.AttributeID) == "" {
			return sharederrors.BadRequest("attribute_id is required for each value")
		}
		vo, err := s.repo.GetProductTypeAttributeVariantOnly(ctx, ptypeID, pair.AttributeID, tenantID, regionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sharederrors.BadRequest("attribute is not assigned to this product type")
			}
			return err
		}
		if vo {
			return sharederrors.BadRequest("use variant attribute-values endpoint for variant_only attributes")
		}
		attr, err := s.repo.GetCatalogAttribute(ctx, tenantID, regionID, pair.AttributeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sharederrors.NotFound("attribute not found")
			}
			return err
		}
		canonical, err := validateAttributeValue(attr.InputType, pair.Value, attr.AllowedValues)
		if err != nil {
			return err
		}
		normalized = append(normalized, AttributeValuePair{AttributeID: pair.AttributeID, Value: canonical})
	}
	return s.repo.SetProductAttributeValues(ctx, productID, normalized)
}

func (s *Service) ListVariantAttributeValues(ctx context.Context, tenantID, regionID, productID, variantID string) ([]AttributeValuePair, error) {
	if err := s.assertVariantInProductRegion(ctx, tenantID, regionID, productID, variantID); err != nil {
		return nil, err
	}
	return s.repo.ListVariantAttributeValues(ctx, variantID)
}

func (s *Service) SetVariantAttributeValues(ctx context.Context, tenantID, regionID, productID, variantID string, pairs []AttributeValuePair) error {
	if err := s.assertVariantInProductRegion(ctx, tenantID, regionID, productID, variantID); err != nil {
		return err
	}
	_, ptypeID, hasType, err := s.repo.GetProductRegionAndType(ctx, tenantID, productID)
	if err != nil {
		return err
	}
	if !hasType {
		return sharederrors.BadRequest("product must have product_type_id to set variant attribute values")
	}
	normalized := make([]AttributeValuePair, 0, len(pairs))
	for _, pair := range pairs {
		if strings.TrimSpace(pair.AttributeID) == "" {
			return sharederrors.BadRequest("attribute_id is required for each value")
		}
		vo, err := s.repo.GetProductTypeAttributeVariantOnly(ctx, ptypeID, pair.AttributeID, tenantID, regionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sharederrors.BadRequest("attribute is not assigned to this product type")
			}
			return err
		}
		if !vo {
			return sharederrors.BadRequest("attribute is not variant_only; set it on the product instead")
		}
		attr, err := s.repo.GetCatalogAttribute(ctx, tenantID, regionID, pair.AttributeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sharederrors.NotFound("attribute not found")
			}
			return err
		}
		canonical, err := validateAttributeValue(attr.InputType, pair.Value, attr.AllowedValues)
		if err != nil {
			return err
		}
		normalized = append(normalized, AttributeValuePair{AttributeID: pair.AttributeID, Value: canonical})
	}
	return s.repo.SetVariantAttributeValues(ctx, variantID, normalized)
}

func (s *Service) assertProductInRegion(ctx context.Context, tenantID, regionID, productID string) error {
	reg, _, _, err := s.repo.GetProductRegionAndType(ctx, tenantID, productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("product not found")
		}
		return err
	}
	if reg != regionID {
		return sharederrors.NotFound("product not found")
	}
	return nil
}

func (s *Service) assertVariantInProductRegion(ctx context.Context, tenantID, regionID, productID, variantID string) error {
	pid, reg, err := s.repo.GetVariantProductRegion(ctx, tenantID, variantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sharederrors.NotFound("variant not found")
		}
		return err
	}
	if reg != regionID || pid != productID {
		return sharederrors.NotFound("variant not found")
	}
	return nil
}
