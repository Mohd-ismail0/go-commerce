package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/sqlc-dev/pqtype"
	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Upsert(ctx context.Context, product Product, idempotencyKey string) (Product, error)
	List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error)
	ListProductTranslations(ctx context.Context, tenantID, regionID string, productIDs []string, languageCode string) (map[string]map[string]string, error)
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
}

func (r *PostgresRepository) ListProductTranslations(ctx context.Context, tenantID, regionID string, productIDs []string, languageCode string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if languageCode == "" || len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT entity_id, fields
FROM translations
WHERE tenant_id = $1
  AND region_id = $2
  AND entity_type = 'product'
  AND language_code = $3
`, tenantID, regionID, languageCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	for _, id := range productIDs {
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
				return Product{
					ID:         existing.ID,
					TenantID:   existing.TenantID,
					RegionID:   existing.RegionID,
					SKU:        existing.Sku,
					Name:       existing.Name,
					Slug:       existing.Slug.String,
					Description: existing.Description.String,
					SEOTitle:   existing.SeoTitle.String,
					SEODescription: existing.SeoDescription.String,
					Metadata:   metadataString(existing.Metadata),
					ExternalReference: existing.ExternalReference.String,
					Currency:   existing.Currency,
					PriceCents: existing.PriceCents,
					CreatedAt:  existing.CreatedAt.UTC().Format(time.RFC3339Nano),
				}, nil
			}
		}
	}
	row, err := r.queries.UpsertProduct(ctx, dbsqlc.UpsertProductParams{
		ID:         product.ID,
		TenantID:   product.TenantID,
		RegionID:   product.RegionID,
		Sku:        product.SKU,
		Name:       product.Name,
		Slug:       nullString(product.Slug),
		Description: nullString(product.Description),
		SeoTitle:   nullString(product.SEOTitle),
		SeoDescription: nullString(product.SEODescription),
		Metadata:   nullRawMessage(product.Metadata),
		ExternalReference: nullString(product.ExternalReference),
		Currency:   product.Currency,
		PriceCents: product.PriceCents,
	})
	if err != nil {
		return Product{}, err
	}
	if idempotencyKey != "" {
		_ = r.queries.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
			TenantID:       product.TenantID,
			Scope:          "products.upsert",
			IdempotencyKey: idempotencyKey,
			ResourceID:     row.ID,
		})
	}
	return Product{
		ID:         row.ID,
		TenantID:   row.TenantID,
		RegionID:   row.RegionID,
		SKU:        row.Sku,
		Name:       row.Name,
		Slug:       row.Slug.String,
		Description: row.Description.String,
		SEOTitle:   row.SeoTitle.String,
		SEODescription: row.SeoDescription.String,
		Metadata:   metadataString(row.Metadata),
		ExternalReference: row.ExternalReference.String,
		Currency:   row.Currency,
		PriceCents: row.PriceCents,
		CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
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
		out = append(out, Product{
			ID:         row.ID,
			TenantID:   row.TenantID,
			RegionID:   row.RegionID,
			SKU:        row.Sku,
			Name:       row.Name,
			Slug:       row.Slug.String,
			Description: row.Description.String,
			SEOTitle:   row.SeoTitle.String,
			SEODescription: row.SeoDescription.String,
			Metadata:   metadataString(row.Metadata),
			ExternalReference: row.ExternalReference.String,
			Currency:   row.Currency,
			PriceCents: row.PriceCents,
			CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return out, nil
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

func (r *PostgresRepository) IsSKUTenantRegionAvailable(ctx context.Context, tenantID, regionID, sku, variantID string) (bool, error) {
	exists, err := r.queries.SkuExistsInTenantRegion(ctx, dbsqlc.SkuExistsInTenantRegionParams{
		TenantID:  tenantID,
		RegionID:  regionID,
		Sku:       sku,
		Column4:   variantID,
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
