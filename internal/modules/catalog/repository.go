package catalog

import (
	"context"
	"database/sql"
	"time"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Upsert(ctx context.Context, product Product, idempotencyKey string) (Product, error)
	List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error)
}

type PostgresRepository struct {
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) Upsert(ctx context.Context, product Product, idempotencyKey string) (Product, error) {
	if idempotencyKey != "" {
		resourceID, err := r.queries.GetIdempotencyResource(ctx, product.TenantID, "products.upsert", idempotencyKey)
		if err == nil && resourceID != "" {
			existing, getErr := r.queries.GetProductByID(ctx, product.TenantID, resourceID)
			if getErr == nil {
				return Product{
					ID:         existing.ID,
					TenantID:   existing.TenantID,
					RegionID:   existing.RegionID,
					SKU:        existing.Sku,
					Name:       existing.Name,
					Currency:   existing.Currency,
					PriceCents: existing.PriceCents,
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
		Currency:   product.Currency,
		PriceCents: product.PriceCents,
	})
	if err != nil {
		return Product{}, err
	}
	if idempotencyKey != "" {
		_ = r.queries.SaveIdempotencyResource(ctx, product.TenantID, "products.upsert", idempotencyKey, row.ID)
	}
	return Product{
		ID:         row.ID,
		TenantID:   row.TenantID,
		RegionID:   row.RegionID,
		SKU:        row.Sku,
		Name:       row.Name,
		Currency:   row.Currency,
		PriceCents: row.PriceCents,
	}, nil
}

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID, sku string, cursor *time.Time, limit int32) ([]Product, error) {
	rows, err := r.queries.ListProductsByTenantRegion(ctx, dbsqlc.ListProductsByTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
		Sku:      sku,
		Cursor:   cursor,
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
			Currency:   row.Currency,
			PriceCents: row.PriceCents,
		})
	}
	return out, nil
}
