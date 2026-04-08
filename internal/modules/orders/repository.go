package orders

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

type Repository interface {
	Insert(ctx context.Context, order Order, idempotencyKey string) (Order, error)
	GetByID(ctx context.Context, tenantID, orderID string) (Order, error)
	UpdateStatus(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error)
	List(ctx context.Context, tenantID, regionID string, cursor *time.Time, limit int32) ([]Order, error)
}

type PostgresRepository struct {
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) Insert(ctx context.Context, order Order, idempotencyKey string) (Order, error) {
	if idempotencyKey != "" {
		resourceID, err := r.queries.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
			TenantID:       order.TenantID,
			Scope:          "orders.create",
			IdempotencyKey: idempotencyKey,
		})
		if err == nil && resourceID != "" {
			existing, getErr := r.queries.GetOrderByID(ctx, dbsqlc.GetOrderByIDParams{
				ID:       resourceID,
				TenantID: order.TenantID,
			})
			if getErr == nil {
				return mapOrder(existing), nil
			}
		}
	}
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
	if idempotencyKey != "" {
		_ = r.queries.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
			TenantID:       order.TenantID,
			Scope:          "orders.create",
			IdempotencyKey: idempotencyKey,
			ResourceID:     row.ID,
		})
	}
	return mapOrder(row), nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	current, err := r.queries.GetOrderByID(ctx, dbsqlc.GetOrderByIDParams{
		ID:       input.ID,
		TenantID: tenantID,
	})
	if err != nil {
		return Order{}, err
	}
	row, err := r.queries.UpdateOrderStatus(ctx, dbsqlc.UpdateOrderStatusParams{
		ID:       input.ID,
		TenantID: tenantID,
		Status:   input.Status,
		UpdatedAt: input.ExpectedUpdatedAt,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrOptimisticLockFailed
		}
		return Order{}, err
	}
	_ = r.queries.InsertOrderAudit(ctx, dbsqlc.InsertOrderAuditParams{
		TenantID:       tenantID,
		RegionID:       row.RegionID,
		OrderID:        row.ID,
		PreviousStatus: current.Status,
		NewStatus:      row.Status,
	})
	return mapOrder(row), nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, orderID string) (Order, error) {
	row, err := r.queries.GetOrderByID(ctx, dbsqlc.GetOrderByIDParams{
		ID:       orderID,
		TenantID: tenantID,
	})
	if err != nil {
		return Order{}, err
	}
	return mapOrder(row), nil
}

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID string, cursor *time.Time, limit int32) ([]Order, error) {
	rows, err := r.queries.ListOrdersByTenantRegion(ctx, dbsqlc.ListOrdersByTenantRegionParams{
		TenantID: tenantID,
		Column2:  regionID,
		Column3:  derefTime(cursor),
		Limit:    limit,
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

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
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
		UpdatedAt:  row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
