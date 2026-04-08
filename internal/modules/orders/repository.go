package orders

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbsqlc "rewrite/internal/shared/db/sqlc"
	"rewrite/internal/shared/utils"
)

type Repository interface {
	Insert(ctx context.Context, order Order, idempotencyKey string) (Order, error)
	GetByID(ctx context.Context, tenantID, orderID string) (Order, error)
	UpdateStatus(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error)
	UpdateStatusAndRestock(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error)
	List(ctx context.Context, tenantID, regionID string, cursor *time.Time, limit int32) ([]Order, error)
}

type PostgresRepository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn, queries: dbsqlc.New(conn)}
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

func (r *PostgresRepository) UpdateStatusAndRestock(ctx context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var previousStatus string
	row := tx.QueryRowContext(ctx, `
SELECT status FROM orders
WHERE id = $1 AND tenant_id = $2
FOR UPDATE
`, input.ID, tenantID)
	if err := row.Scan(&previousStatus); err != nil {
		return Order{}, err
	}

	updated := tx.QueryRowContext(ctx, `
UPDATE orders
SET status = $3, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND updated_at = $4
RETURNING id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
`, input.ID, tenantID, input.Status, input.ExpectedUpdatedAt)
	var mapped dbsqlc.Order
	if err := updated.Scan(
		&mapped.ID,
		&mapped.TenantID,
		&mapped.RegionID,
		&mapped.CustomerID,
		&mapped.Status,
		&mapped.TotalCents,
		&mapped.Currency,
		&mapped.CreatedAt,
		&mapped.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrOptimisticLockFailed
		}
		return Order{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO order_status_audit (id, tenant_id, region_id, order_id, previous_status, new_status, changed_at)
VALUES ($1,$2,$3,$4,$5,$6,NOW())
`, utils.NewID("osa"), tenantID, mapped.RegionID, mapped.ID, previousStatus, mapped.Status); err != nil {
		return Order{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE stock_items s
SET quantity = s.quantity + a.quantity, updated_at = NOW()
FROM stock_allocations a
WHERE a.order_id = $1
  AND a.tenant_id = $2
  AND s.id = a.stock_item_id
  AND s.tenant_id = a.tenant_id
  AND s.region_id = a.region_id
`, input.ID, tenantID); err != nil {
		return Order{}, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM stock_allocations
WHERE order_id = $1 AND tenant_id = $2
`, input.ID, tenantID); err != nil {
		return Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return mapOrder(mapped), nil
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
