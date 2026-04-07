package orders

import (
	"context"
	"database/sql"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Insert(ctx context.Context, order Order) (Order, error)
	UpdateStatus(ctx context.Context, tenantID, orderID, status string) (Order, error)
	List(ctx context.Context, tenantID, regionID string) ([]Order, error)
}

type PostgresRepository struct {
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) Insert(ctx context.Context, order Order) (Order, error) {
	row, err := r.queries.InsertOrder(ctx, dbsqlc.InsertOrderParams{
		ID:         order.ID,
		TenantID:   order.TenantID,
		RegionID:   order.RegionID,
		CustomerID: order.CustomerID,
		Status:     order.Status,
		TotalCents: order.TotalCents,
		Currency:   order.Currency,
	})
	if err != nil {
		return Order{}, err
	}
	return mapOrder(row), nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, tenantID, orderID, status string) (Order, error) {
	row, err := r.queries.UpdateOrderStatus(ctx, dbsqlc.UpdateOrderStatusParams{
		ID:       orderID,
		TenantID: tenantID,
		Status:   status,
	})
	if err != nil {
		return Order{}, err
	}
	return mapOrder(row), nil
}

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID string) ([]Order, error) {
	rows, err := r.queries.ListOrdersByTenantRegion(ctx, dbsqlc.ListOrdersByTenantRegionParams{
		TenantID: tenantID,
		RegionID: regionID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Order, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOrder(row))
	}
	return out, nil
}

func mapOrder(row dbsqlc.Order) Order {
	return Order{
		ID:         row.ID,
		TenantID:   row.TenantID,
		RegionID:   row.RegionID,
		CustomerID: row.CustomerID,
		Status:     row.Status,
		TotalCents: row.TotalCents,
		Currency:   row.Currency,
	}
}
