package catalog

import (
	"context"
	"database/sql"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Upsert(ctx context.Context, product Product) (Product, error)
	List(ctx context.Context, tenantID, regionID, sku string) ([]Product, error)
}

type PostgresRepository struct {
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) Upsert(ctx context.Context, product Product) (Product, error) {
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

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID, sku string) ([]Product, error) {
	rows, err := r.queries.ListProductsByTenantRegion(ctx, dbsqlc.ListProductsByTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
		Sku:      sku,
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
