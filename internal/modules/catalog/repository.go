package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/sqlc-dev/pqtype"
	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Upsert(ctx context.Context, product Product, idempotencyKey string) (Product, error)
	List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error)
	ListProductTranslations(ctx context.Context, tenantID, regionID string, productIDs []string, languageCode string) (map[string]map[string]string, error)
	ListCategoryTranslations(ctx context.Context, tenantID, regionID string, categoryIDs []string, languageCode string) (map[string]map[string]string, error)
	ListCollectionTranslations(ctx context.Context, tenantID, regionID string, collectionIDs []string, languageCode string) (map[string]map[string]string, error)
	IsProductSlugAvailable(ctx context.Context, tenantID, regionID, slug, productID string) (bool, error)
	UpsertVariant(ctx context.Context, variant ProductVariant) (ProductVariant, error)
	ListVariants(ctx context.Context, tenantID, regionID, productID string) ([]ProductVariant, error)
	IsSKUTenantRegionAvailable(ctx context.Context, tenantID, regionID, sku, variantID string) (bool, error)
	InsertCategory(ctx context.Context, category Category) (Category, error)
	ListCategories(ctx context.Context, tenantID, regionID string) ([]Category, error)
	InsertCollection(ctx context.Context, collection Collection) (Collection, error)
	ListCollections(ctx context.Context, tenantID, regionID string) ([]Collection, error)
	AssignProductToCollection(ctx context.Context, tenantID, regionID, collectionID, productID string) error
	InsertProductMedia(ctx context.Context, media ProductMedia) (ProductMedia, error)
	ListProductMedia(ctx context.Context, tenantID, regionID, productID string) ([]ProductMedia, error)
	ListProductsByChannel(ctx context.Context, tenantID, regionID, channelID, sku string, onlyPublished bool, cursor *time.Time, limit int32) ([]Product, error)
	ListVariantsByChannel(ctx context.Context, tenantID, regionID, productID, channelID string, onlyPublished bool) ([]ProductVariant, error)

	InsertProductType(ctx context.Context, pt ProductType) (ProductType, error)
	ListProductTypes(ctx context.Context, tenantID, regionID string) ([]ProductType, error)
	GetProductType(ctx context.Context, tenantID, regionID, id string) (ProductType, error)
	ListProductTypeAttributeDefs(ctx context.Context, tenantID, regionID, productTypeID string) ([]ProductTypeAttributeDef, error)
	InsertCatalogAttribute(ctx context.Context, a CatalogAttribute) (CatalogAttribute, error)
	ListCatalogAttributes(ctx context.Context, tenantID, regionID string) ([]CatalogAttribute, error)
	GetCatalogAttribute(ctx context.Context, tenantID, regionID, id string) (CatalogAttribute, error)
	LinkAttributeToProductType(ctx context.Context, productTypeID string, in LinkAttributeToTypeInput) error
	UnlinkAttributeFromProductType(ctx context.Context, tenantID, regionID, productTypeID, attributeID string) error
	ListProductAttributeValues(ctx context.Context, productID string) ([]AttributeValuePair, error)
	ListVariantAttributeValues(ctx context.Context, variantID string) ([]AttributeValuePair, error)
	ListProductAttributeValuesForProducts(ctx context.Context, productIDs []string) (map[string][]AttributeValuePair, error)
	SetProductAttributeValues(ctx context.Context, productID string, pairs []AttributeValuePair) error
	SetVariantAttributeValues(ctx context.Context, variantID string, pairs []AttributeValuePair) error
	GetProductRegionAndType(ctx context.Context, tenantID, productID string) (regionID string, productTypeID string, hasType bool, err error)
	GetVariantProductRegion(ctx context.Context, tenantID, variantID string) (productID, regionID string, err error)
	GetProductTypeAttributeVariantOnly(ctx context.Context, productTypeID, attributeID, tenantID, regionID string) (variantOnly bool, err error)
}

func (r *PostgresRepository) ListProductTranslations(ctx context.Context, tenantID, regionID string, productIDs []string, languageCode string) (map[string]map[string]string, error) {
	return r.listEntityTranslations(ctx, tenantID, regionID, "product", productIDs, languageCode)
}

func (r *PostgresRepository) ListCategoryTranslations(ctx context.Context, tenantID, regionID string, categoryIDs []string, languageCode string) (map[string]map[string]string, error) {
	return r.listEntityTranslations(ctx, tenantID, regionID, "category", categoryIDs, languageCode)
}

func (r *PostgresRepository) ListCollectionTranslations(ctx context.Context, tenantID, regionID string, collectionIDs []string, languageCode string) (map[string]map[string]string, error) {
	return r.listEntityTranslations(ctx, tenantID, regionID, "collection", collectionIDs, languageCode)
}

func (r *PostgresRepository) listEntityTranslations(ctx context.Context, tenantID, regionID, entityType string, entityIDs []string, languageCode string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if languageCode == "" || len(entityIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT entity_id, fields
FROM translations
WHERE tenant_id = $1
  AND region_id = $2
  AND entity_type = $3
  AND language_code = $4
`, tenantID, regionID, entityType, languageCode)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var entityID string
		var fieldsRaw []byte
		if err := rows.Scan(&entityID, &fieldsRaw); err != nil {
			return nil, err
		}
		fields := map[string]string{}
		var generic map[string]any
		if err := json.Unmarshal(fieldsRaw, &generic); err == nil {
			for k, v := range generic {
				if s, ok := v.(string); ok {
					fields[k] = s
				}
			}
		}
		out[entityID] = fields
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	allowed := map[string]struct{}{}
	for _, id := range entityIDs {
		allowed[id] = struct{}{}
	}
	filtered := map[string]map[string]string{}
	for id, fields := range out {
		if _, ok := allowed[id]; ok {
			filtered[id] = fields
		}
	}
	return filtered, nil
}

type PostgresRepository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn, queries: dbsqlc.New(conn)}
}

var ErrAssignEntityNotFound = errors.New("collection or product not found in tenant/region")
var ErrCollectionProductAlreadyAssigned = errors.New("product already assigned to collection")

func productFromSQLC(row dbsqlc.Product) Product {
	p := Product{
		ID:                row.ID,
		TenantID:          row.TenantID,
		RegionID:          row.RegionID,
		SKU:               row.Sku,
		Name:              row.Name,
		Slug:              row.Slug.String,
		Description:       row.Description.String,
		SEOTitle:          row.SeoTitle.String,
		SEODescription:    row.SeoDescription.String,
		Metadata:          metadataString(row.Metadata),
		ExternalReference: row.ExternalReference.String,
		Currency:          row.Currency,
		PriceCents:        row.PriceCents,
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.ProductTypeID.Valid {
		p.ProductTypeID = row.ProductTypeID.String
	}
	return p
}

func (r *PostgresRepository) Upsert(ctx context.Context, product Product, idempotencyKey string) (Product, error) {
	if idempotencyKey != "" {
		resourceID, err := r.queries.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
			TenantID:       product.TenantID,
			Scope:          "products.upsert",
			IdempotencyKey: idempotencyKey,
		})
		if err == nil && resourceID != "" {
			existing, getErr := r.queries.GetProductByID(ctx, dbsqlc.GetProductByIDParams{
				ID:       resourceID,
				TenantID: product.TenantID,
			})
			if getErr == nil {
				return productFromSQLC(existing), nil
			}
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Product{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	qtx := r.queries.WithTx(tx)
	ptypeID := sql.NullString{}
	if strings.TrimSpace(product.ProductTypeID) != "" {
		ptypeID = sql.NullString{String: strings.TrimSpace(product.ProductTypeID), Valid: true}
	}
	row, err := qtx.UpsertProduct(ctx, dbsqlc.UpsertProductParams{
		ID:                product.ID,
		TenantID:          product.TenantID,
		RegionID:          product.RegionID,
		Sku:               product.SKU,
		Name:              product.Name,
		Slug:              nullString(product.Slug),
		Description:       nullString(product.Description),
		SeoTitle:          nullString(product.SEOTitle),
		SeoDescription:    nullString(product.SEODescription),
		Metadata:          nullRawMessage(product.Metadata),
		ExternalReference: nullString(product.ExternalReference),
		Currency:          product.Currency,
		PriceCents:        product.PriceCents,
		ProductTypeID:     ptypeID,
	})
	if err != nil {
		return Product{}, err
	}
	if err := qtx.CleanupProductAttributeValuesForProduct(ctx, dbsqlc.CleanupProductAttributeValuesForProductParams{
		ID:       row.ID,
		TenantID: product.TenantID,
	}); err != nil {
		return Product{}, err
	}
	if err := qtx.CleanupVariantAttributeValuesForProduct(ctx, dbsqlc.CleanupVariantAttributeValuesForProductParams{
		ID:       row.ID,
		TenantID: product.TenantID,
	}); err != nil {
		return Product{}, err
	}
	if idempotencyKey != "" {
		if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
			TenantID:       product.TenantID,
			Scope:          "products.upsert",
			IdempotencyKey: idempotencyKey,
			ResourceID:     row.ID,
		}); err != nil {
			return Product{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Product{}, err
	}
	return productFromSQLC(row), nil
}

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error) {
	rows, err := r.queries.ListProductsByTenantRegion(ctx, dbsqlc.ListProductsByTenantRegionParams{
		TenantID: tenantID,
		Column2:  regionID,
		Column3:  sku,
		Column4:  derefTime(cursor),
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Product, 0, len(rows))
	for _, row := range rows {
		out = append(out, productFromSQLC(row))
	}
	return out, nil
}

func (r *PostgresRepository) ListProductsByChannel(ctx context.Context, tenantID, regionID, channelID, sku string, onlyPublished bool, cursor *time.Time, limit int32) ([]Product, error) {
	query := `
SELECT p.id, p.tenant_id, p.region_id, p.sku, p.name, p.slug, p.description, p.seo_title, p.seo_description, p.metadata, p.external_reference, p.currency, p.price_cents, p.product_type_id, p.created_at
FROM products p
JOIN product_channel_listings pcl
  ON pcl.product_id = p.id
 AND pcl.tenant_id = p.tenant_id
 AND pcl.region_id = p.region_id
WHERE p.tenant_id = $1
  AND p.region_id = $2
  AND pcl.channel_id = $3
  AND ($4::text = '' OR p.sku = $4)
  AND ($5::bool = false OR pcl.is_published = true)
  AND ($6::timestamptz = '0001-01-01 00:00:00+00'::timestamptz OR p.created_at < $6)
ORDER BY p.created_at DESC
LIMIT $7
`
	rows, err := r.db.QueryContext(ctx, query, tenantID, regionID, channelID, sku, onlyPublished, derefTime(cursor), limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Product{}
	for rows.Next() {
		var row Product
		var slug, description, seoTitle, seoDescription, externalRef, productTypeID sql.NullString
		var metadataRaw []byte
		var metadata sql.NullString
		var createdAt time.Time
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.RegionID, &row.SKU, &row.Name, &slug, &description, &seoTitle, &seoDescription,
			&metadataRaw, &externalRef, &row.Currency, &row.PriceCents, &productTypeID, &createdAt,
		); err != nil {
			return nil, err
		}
		row.Slug = slug.String
		row.Description = description.String
		row.SEOTitle = seoTitle.String
		row.SEODescription = seoDescription.String
		if productTypeID.Valid {
			row.ProductTypeID = productTypeID.String
		}
		if len(metadataRaw) > 0 {
			metadata = sql.NullString{String: string(metadataRaw), Valid: true}
		}
		if metadata.Valid {
			row.Metadata = metadata.String
		}
		row.ExternalReference = externalRef.String
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) IsProductSlugAvailable(ctx context.Context, tenantID, regionID, slug, productID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM products
  WHERE tenant_id = $1
    AND region_id = $2
    AND slug = $3
    AND ($4::text = '' OR id <> $4)
)
`, tenantID, regionID, slug, productID).Scan(&exists); err != nil {
		return false, err
	}
	return !exists, nil
}

func (r *PostgresRepository) UpsertVariant(ctx context.Context, variant ProductVariant) (ProductVariant, error) {
	row, err := r.queries.UpsertProductVariant(ctx, dbsqlc.UpsertProductVariantParams{
		ID:         variant.ID,
		TenantID:   variant.TenantID,
		RegionID:   variant.RegionID,
		ProductID:  variant.ProductID,
		Sku:        variant.SKU,
		Name:       variant.Name,
		PriceCents: variant.PriceCents,
		Currency:   variant.Currency,
	})
	if err != nil {
		return ProductVariant{}, err
	}
	return ProductVariant{
		ID:         row.ID,
		TenantID:   row.TenantID,
		RegionID:   row.RegionID,
		ProductID:  row.ProductID,
		SKU:        row.Sku,
		Name:       row.Name,
		PriceCents: row.PriceCents,
		Currency:   row.Currency,
	}, nil
}

func (r *PostgresRepository) ListVariants(ctx context.Context, tenantID, regionID, productID string) ([]ProductVariant, error) {
	rows, err := r.queries.ListProductVariantsByProduct(ctx, dbsqlc.ListProductVariantsByProductParams{
		TenantID:  tenantID,
		RegionID:  regionID,
		ProductID: productID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProductVariant, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProductVariant{
			ID:         row.ID,
			TenantID:   row.TenantID,
			RegionID:   row.RegionID,
			ProductID:  row.ProductID,
			SKU:        row.Sku,
			Name:       row.Name,
			PriceCents: row.PriceCents,
			Currency:   row.Currency,
		})
	}
	return out, nil
}

func (r *PostgresRepository) ListVariantsByChannel(ctx context.Context, tenantID, regionID, productID, channelID string, onlyPublished bool) ([]ProductVariant, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT v.id, v.tenant_id, v.region_id, v.product_id, v.sku, v.name, v.price_cents, v.currency
FROM product_variants v
JOIN variant_channel_listings vcl
  ON vcl.variant_id = v.id
 AND vcl.tenant_id = v.tenant_id
 AND vcl.region_id = v.region_id
WHERE v.tenant_id = $1
  AND v.region_id = $2
  AND v.product_id = $3
  AND vcl.channel_id = $4
  AND ($5::bool = false OR vcl.is_published = true)
ORDER BY v.id
`, tenantID, regionID, productID, channelID, onlyPublished)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []ProductVariant{}
	for rows.Next() {
		var row ProductVariant
		if err := rows.Scan(&row.ID, &row.TenantID, &row.RegionID, &row.ProductID, &row.SKU, &row.Name, &row.PriceCents, &row.Currency); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) IsSKUTenantRegionAvailable(ctx context.Context, tenantID, regionID, sku, variantID string) (bool, error) {
	exists, err := r.queries.SkuExistsInTenantRegion(ctx, dbsqlc.SkuExistsInTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
		Sku:      sku,
		Column4:  variantID,
	})
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func nullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

func nullRawMessage(v string) pqtype.NullRawMessage {
	if v == "" {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: []byte(v), Valid: true}
}

func metadataString(v pqtype.NullRawMessage) string {
	if !v.Valid {
		return ""
	}
	return string(v.RawMessage)
}

func (r *PostgresRepository) InsertCategory(ctx context.Context, category Category) (Category, error) {
	row, err := r.queries.InsertCategory(ctx, dbsqlc.InsertCategoryParams{
		ID:       category.ID,
		TenantID: category.TenantID,
		RegionID: category.RegionID,
		Name:     category.Name,
		Slug:     category.Slug,
		ParentID: sql.NullString{String: category.ParentID, Valid: category.ParentID != ""},
	})
	if err != nil {
		return Category{}, err
	}
	return Category{
		ID:       row.ID,
		TenantID: row.TenantID,
		RegionID: row.RegionID,
		Name:     row.Name,
		Slug:     row.Slug,
		ParentID: row.ParentID.String,
	}, nil
}

func (r *PostgresRepository) ListCategories(ctx context.Context, tenantID, regionID string) ([]Category, error) {
	rows, err := r.queries.ListCategoriesByTenantRegion(ctx, dbsqlc.ListCategoriesByTenantRegionParams{TenantID: tenantID, RegionID: regionID})
	if err != nil {
		return nil, err
	}
	out := make([]Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, Category{ID: row.ID, TenantID: row.TenantID, RegionID: row.RegionID, Name: row.Name, Slug: row.Slug, ParentID: row.ParentID.String})
	}
	return out, nil
}

func (r *PostgresRepository) InsertCollection(ctx context.Context, collection Collection) (Collection, error) {
	row, err := r.queries.InsertCollection(ctx, dbsqlc.InsertCollectionParams{
		ID:       collection.ID,
		TenantID: collection.TenantID,
		RegionID: collection.RegionID,
		Name:     collection.Name,
		Slug:     collection.Slug,
	})
	if err != nil {
		return Collection{}, err
	}
	return Collection{ID: row.ID, TenantID: row.TenantID, RegionID: row.RegionID, Name: row.Name, Slug: row.Slug}, nil
}

func (r *PostgresRepository) ListCollections(ctx context.Context, tenantID, regionID string) ([]Collection, error) {
	rows, err := r.queries.ListCollectionsByTenantRegion(ctx, dbsqlc.ListCollectionsByTenantRegionParams{TenantID: tenantID, RegionID: regionID})
	if err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(rows))
	for _, row := range rows {
		out = append(out, Collection{ID: row.ID, TenantID: row.TenantID, RegionID: row.RegionID, Name: row.Name, Slug: row.Slug})
	}
	return out, nil
}

func (r *PostgresRepository) AssignProductToCollection(ctx context.Context, tenantID, regionID, collectionID, productID string) error {
	var collectionExists bool
	if err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM collections
  WHERE id = $1 AND tenant_id = $2 AND region_id = $3
)
`, collectionID, tenantID, regionID).Scan(&collectionExists); err != nil {
		return err
	}
	var productExists bool
	if err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM products
  WHERE id = $1 AND tenant_id = $2 AND region_id = $3
)
`, productID, tenantID, regionID).Scan(&productExists); err != nil {
		return err
	}
	if !collectionExists || !productExists {
		return ErrAssignEntityNotFound
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO collection_products (collection_id, product_id)
VALUES ($1, $2)
ON CONFLICT (collection_id, product_id) DO NOTHING
`, collectionID, productID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrCollectionProductAlreadyAssigned
	}
	return nil
}

func (r *PostgresRepository) InsertProductMedia(ctx context.Context, media ProductMedia) (ProductMedia, error) {
	row, err := r.queries.InsertProductMedia(ctx, dbsqlc.InsertProductMediaParams{
		ID:        media.ID,
		TenantID:  media.TenantID,
		RegionID:  media.RegionID,
		ProductID: media.ProductID,
		Url:       media.URL,
		MediaType: media.MediaType,
	})
	if err != nil {
		return ProductMedia{}, err
	}
	return ProductMedia{
		ID:        row.ID,
		TenantID:  row.TenantID,
		RegionID:  row.RegionID,
		ProductID: row.ProductID,
		URL:       row.Url,
		MediaType: row.MediaType,
	}, nil
}

func (r *PostgresRepository) ListProductMedia(ctx context.Context, tenantID, regionID, productID string) ([]ProductMedia, error) {
	rows, err := r.queries.ListProductMediaByProduct(ctx, dbsqlc.ListProductMediaByProductParams{
		TenantID:  tenantID,
		RegionID:  regionID,
		ProductID: productID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProductMedia, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProductMedia{
			ID:        row.ID,
			TenantID:  row.TenantID,
			RegionID:  row.RegionID,
			ProductID: row.ProductID,
			URL:       row.Url,
			MediaType: row.MediaType,
		})
	}
	return out, nil
}

func productTypeFromSQLC(row dbsqlc.ProductType) ProductType {
	return ProductType{
		ID:        row.ID,
		TenantID:  row.TenantID,
		RegionID:  row.RegionID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func catalogAttrFromSQLC(row dbsqlc.CatalogAttribute) CatalogAttribute {
	a := CatalogAttribute{
		ID:        row.ID,
		TenantID:  row.TenantID,
		RegionID:  row.RegionID,
		Name:      row.Name,
		Slug:      row.Slug,
		InputType: row.InputType,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.Unit.Valid {
		a.Unit = row.Unit.String
	}
	if row.AllowedValues.Valid && len(row.AllowedValues.RawMessage) > 0 {
		a.AllowedValues = json.RawMessage(row.AllowedValues.RawMessage)
	}
	return a
}

func (r *PostgresRepository) InsertProductType(ctx context.Context, pt ProductType) (ProductType, error) {
	row, err := r.queries.InsertProductType(ctx, dbsqlc.InsertProductTypeParams{
		ID:       pt.ID,
		TenantID: pt.TenantID,
		RegionID: pt.RegionID,
		Name:     pt.Name,
		Slug:     pt.Slug,
	})
	if err != nil {
		return ProductType{}, err
	}
	return productTypeFromSQLC(row), nil
}

func (r *PostgresRepository) ListProductTypes(ctx context.Context, tenantID, regionID string) ([]ProductType, error) {
	rows, err := r.queries.ListProductTypesByTenantRegion(ctx, dbsqlc.ListProductTypesByTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProductType, 0, len(rows))
	for _, row := range rows {
		out = append(out, productTypeFromSQLC(row))
	}
	return out, nil
}

func (r *PostgresRepository) GetProductType(ctx context.Context, tenantID, regionID, id string) (ProductType, error) {
	row, err := r.queries.GetProductTypeByID(ctx, dbsqlc.GetProductTypeByIDParams{
		ID:       id,
		TenantID: tenantID,
		RegionID: regionID,
	})
	if err != nil {
		return ProductType{}, err
	}
	return productTypeFromSQLC(row), nil
}

func (r *PostgresRepository) ListProductTypeAttributeDefs(ctx context.Context, tenantID, regionID, productTypeID string) ([]ProductTypeAttributeDef, error) {
	rows, err := r.queries.ListProductTypeAttributeRows(ctx, dbsqlc.ListProductTypeAttributeRowsParams{
		ProductTypeID: productTypeID,
		TenantID:      tenantID,
		RegionID:      regionID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProductTypeAttributeDef, 0, len(rows))
	for _, row := range rows {
		d := ProductTypeAttributeDef{
			AttributeID: row.AttributeID,
			Name:        row.AttributeName,
			Slug:        row.AttributeSlug,
			InputType:   row.InputType,
			SortOrder:   int(row.SortOrder),
			VariantOnly: row.VariantOnly,
		}
		if row.Unit.Valid {
			d.Unit = row.Unit.String
		}
		if row.AllowedValues.Valid && len(row.AllowedValues.RawMessage) > 0 {
			d.AllowedValues = json.RawMessage(row.AllowedValues.RawMessage)
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *PostgresRepository) InsertCatalogAttribute(ctx context.Context, a CatalogAttribute) (CatalogAttribute, error) {
	unit := sql.NullString{}
	if strings.TrimSpace(a.Unit) != "" {
		unit = sql.NullString{String: strings.TrimSpace(a.Unit), Valid: true}
	}
	allowed := pqtype.NullRawMessage{}
	if len(a.AllowedValues) > 0 {
		allowed = pqtype.NullRawMessage{RawMessage: []byte(a.AllowedValues), Valid: true}
	}
	row, err := r.queries.InsertCatalogAttribute(ctx, dbsqlc.InsertCatalogAttributeParams{
		ID:            a.ID,
		TenantID:      a.TenantID,
		RegionID:      a.RegionID,
		Name:          a.Name,
		Slug:          a.Slug,
		InputType:     a.InputType,
		Unit:          unit,
		AllowedValues: allowed,
	})
	if err != nil {
		return CatalogAttribute{}, err
	}
	return catalogAttrFromSQLC(row), nil
}

func (r *PostgresRepository) ListCatalogAttributes(ctx context.Context, tenantID, regionID string) ([]CatalogAttribute, error) {
	rows, err := r.queries.ListCatalogAttributesByTenantRegion(ctx, dbsqlc.ListCatalogAttributesByTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]CatalogAttribute, 0, len(rows))
	for _, row := range rows {
		out = append(out, catalogAttrFromSQLC(row))
	}
	return out, nil
}

func (r *PostgresRepository) GetCatalogAttribute(ctx context.Context, tenantID, regionID, id string) (CatalogAttribute, error) {
	row, err := r.queries.GetCatalogAttributeByID(ctx, dbsqlc.GetCatalogAttributeByIDParams{
		ID:       id,
		TenantID: tenantID,
		RegionID: regionID,
	})
	if err != nil {
		return CatalogAttribute{}, err
	}
	return catalogAttrFromSQLC(row), nil
}

func (r *PostgresRepository) LinkAttributeToProductType(ctx context.Context, productTypeID string, in LinkAttributeToTypeInput) error {
	return r.queries.LinkAttributeToProductType(ctx, dbsqlc.LinkAttributeToProductTypeParams{
		ProductTypeID: productTypeID,
		AttributeID:   in.AttributeID,
		SortOrder:     int32(in.SortOrder),
		VariantOnly:   in.VariantOnly,
	})
}

func (r *PostgresRepository) UnlinkAttributeFromProductType(ctx context.Context, tenantID, regionID, productTypeID, attributeID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := r.queries.WithTx(tx)
	if _, err := qtx.GetProductTypeByID(ctx, dbsqlc.GetProductTypeByIDParams{
		ID:       productTypeID,
		TenantID: tenantID,
		RegionID: regionID,
	}); err != nil {
		return err
	}
	if err := qtx.DeleteProductAttributeValuesForUnlinkedAttribute(ctx, dbsqlc.DeleteProductAttributeValuesForUnlinkedAttributeParams{
		ProductTypeID: productTypeID,
		AttributeID:   attributeID,
	}); err != nil {
		return err
	}
	if err := qtx.DeleteVariantAttributeValuesForUnlinkedAttribute(ctx, dbsqlc.DeleteVariantAttributeValuesForUnlinkedAttributeParams{
		ProductTypeID: productTypeID,
		AttributeID:   attributeID,
	}); err != nil {
		return err
	}
	if err := qtx.UnlinkAttributeFromProductType(ctx, dbsqlc.UnlinkAttributeFromProductTypeParams{
		ProductTypeID: productTypeID,
		AttributeID:   attributeID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) ListProductAttributeValues(ctx context.Context, productID string) ([]AttributeValuePair, error) {
	rows, err := r.queries.ListProductAttributeValues(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]AttributeValuePair, 0, len(rows))
	for _, row := range rows {
		out = append(out, AttributeValuePair{AttributeID: row.AttributeID, Value: row.ValueText})
	}
	return out, nil
}

func (r *PostgresRepository) ListVariantAttributeValues(ctx context.Context, variantID string) ([]AttributeValuePair, error) {
	rows, err := r.queries.ListVariantAttributeValues(ctx, variantID)
	if err != nil {
		return nil, err
	}
	out := make([]AttributeValuePair, 0, len(rows))
	for _, row := range rows {
		out = append(out, AttributeValuePair{AttributeID: row.AttributeID, Value: row.ValueText})
	}
	return out, nil
}

func (r *PostgresRepository) ListProductAttributeValuesForProducts(ctx context.Context, productIDs []string) (map[string][]AttributeValuePair, error) {
	out := map[string][]AttributeValuePair{}
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.queries.ListProductAttributeValuesForProducts(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProductID] = append(out[row.ProductID], AttributeValuePair{
			AttributeID: row.AttributeID,
			Value:       row.ValueText,
		})
	}
	return out, nil
}

func (r *PostgresRepository) SetProductAttributeValues(ctx context.Context, productID string, pairs []AttributeValuePair) error {
	if len(pairs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := r.queries.WithTx(tx)
	for _, p := range pairs {
		if err := qtx.UpsertProductAttributeValue(ctx, dbsqlc.UpsertProductAttributeValueParams{
			ProductID:   productID,
			AttributeID: p.AttributeID,
			ValueText:   p.Value,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) SetVariantAttributeValues(ctx context.Context, variantID string, pairs []AttributeValuePair) error {
	if len(pairs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := r.queries.WithTx(tx)
	for _, p := range pairs {
		if err := qtx.UpsertVariantAttributeValue(ctx, dbsqlc.UpsertVariantAttributeValueParams{
			VariantID:   variantID,
			AttributeID: p.AttributeID,
			ValueText:   p.Value,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) GetProductRegionAndType(ctx context.Context, tenantID, productID string) (regionID string, productTypeID string, hasType bool, err error) {
	row, err := r.queries.GetProductRegionAndType(ctx, dbsqlc.GetProductRegionAndTypeParams{
		ID:       productID,
		TenantID: tenantID,
	})
	if err != nil {
		return "", "", false, err
	}
	if row.ProductTypeID.Valid {
		return row.RegionID, row.ProductTypeID.String, true, nil
	}
	return row.RegionID, "", false, nil
}

func (r *PostgresRepository) GetVariantProductRegion(ctx context.Context, tenantID, variantID string) (productID, regionID string, err error) {
	row, err := r.queries.GetVariantProductRegion(ctx, dbsqlc.GetVariantProductRegionParams{
		ID:       variantID,
		TenantID: tenantID,
	})
	if err != nil {
		return "", "", err
	}
	return row.ProductID, row.RegionID, nil
}

func (r *PostgresRepository) GetProductTypeAttributeVariantOnly(ctx context.Context, productTypeID, attributeID, tenantID, regionID string) (variantOnly bool, err error) {
	return r.queries.GetProductTypeAttributeAssignment(ctx, dbsqlc.GetProductTypeAttributeAssignmentParams{
		ProductTypeID: productTypeID,
		AttributeID:   attributeID,
		TenantID:      tenantID,
		RegionID:      regionID,
	})
}
