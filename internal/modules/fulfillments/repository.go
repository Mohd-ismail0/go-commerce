package fulfillments

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"rewrite/internal/modules/orders"
	dbsqlc "rewrite/internal/shared/db/sqlc"
)

var ErrOrderNotFound = errors.New("order not found")
var ErrOrderNotFulfillable = errors.New("order cannot be fulfilled from current status")

type Repository interface {
	Create(ctx context.Context, in Fulfillment, idempotencyKey string) (CreateResult, error)
	List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error)
}

type PostgresRepository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn, queries: dbsqlc.New(conn)}
}

func (r *PostgresRepository) Create(ctx context.Context, in Fulfillment, idempotencyKey string) (CreateResult, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key != "" {
		resourceID, err := r.queries.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
			TenantID:       in.TenantID,
			Scope:          "fulfillments.create",
			IdempotencyKey: key,
		})
		if err == nil && resourceID != "" {
			f, err := r.getFulfillmentByID(ctx, in.TenantID, in.RegionID, resourceID)
			if err != nil {
				return CreateResult{}, err
			}
			oRow, err := r.queries.GetOrderByID(ctx, dbsqlc.GetOrderByIDParams{
				ID:       f.OrderID,
				TenantID: in.TenantID,
			})
			if err != nil {
				return CreateResult{}, err
			}
			return CreateResult{
				Fulfillment:           f,
				FinalOrder:            mapOrderRow(oRow),
				FromIdempotencyReplay: true,
				EmitOrderCompleted:    false,
			}, nil
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CreateResult{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var oid, otid, orid, ocid, ostatus string
	var ototal int64
	var ocurr string
	var ocat, ouat time.Time
	err = tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, status, total_cents, currency, created_at, updated_at
FROM orders
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
FOR UPDATE
`, in.OrderID, in.TenantID, in.RegionID).Scan(&oid, &otid, &orid, &ocid, &ostatus, &ototal, &ocurr, &ocat, &ouat)
	if errors.Is(err, sql.ErrNoRows) {
		return CreateResult{}, ErrOrderNotFound
	}
	if err != nil {
		return CreateResult{}, err
	}

	cur := dbsqlc.Order{
		ID:         oid,
		TenantID:   otid,
		RegionID:   orid,
		CustomerID: ocid,
		Status:     ostatus,
		TotalCents: ototal,
		Currency:   ocurr,
		CreatedAt:  ocat,
		UpdatedAt:  ouat,
	}

	if cur.Status == "cancelled" {
		return CreateResult{}, ErrOrderNotFulfillable
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO fulfillments (id, tenant_id, region_id, order_id, status, tracking_number, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NOW(),NOW())
RETURNING id, tenant_id, region_id, order_id, status, COALESCE(tracking_number,''), updated_at
`, in.ID, in.TenantID, in.RegionID, in.OrderID, in.Status, in.TrackingNumber)
	saved, err := scanFulfillment(row)
	if err != nil {
		return CreateResult{}, err
	}

	qtx := r.queries.WithTx(tx)
	emitOrderCompleted := false
	for _, nextStatus := range fulfillmentOrderTransitions(cur.Status) {
		if !orders.ValidStatusTransition(cur.Status, nextStatus) {
			return CreateResult{}, ErrOrderNotFulfillable
		}
		if nextStatus == "completed" && cur.Status != "completed" {
			emitOrderCompleted = true
		}
		updated, err := qtx.UpdateOrderStatus(ctx, dbsqlc.UpdateOrderStatusParams{
			ID:        cur.ID,
			TenantID:  in.TenantID,
			Status:    nextStatus,
			UpdatedAt: cur.UpdatedAt,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return CreateResult{}, orders.ErrOptimisticLockFailed
			}
			return CreateResult{}, err
		}
		if err := qtx.InsertOrderAudit(ctx, dbsqlc.InsertOrderAuditParams{
			TenantID:       in.TenantID,
			RegionID:       cur.RegionID,
			OrderID:        cur.ID,
			PreviousStatus: cur.Status,
			NewStatus:      nextStatus,
		}); err != nil {
			return CreateResult{}, err
		}
		cur = updated
	}

	if key != "" {
		if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
			TenantID:       in.TenantID,
			Scope:          "fulfillments.create",
			IdempotencyKey: key,
			ResourceID:     saved.ID,
		}); err != nil {
			return CreateResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return CreateResult{}, err
	}

	return CreateResult{
		Fulfillment:           saved,
		FinalOrder:            mapOrderRow(cur),
		FromIdempotencyReplay: false,
		EmitOrderCompleted:    emitOrderCompleted,
	}, nil
}

func fulfillmentOrderTransitions(current string) []string {
	switch current {
	case "created":
		return []string{"confirmed", "completed"}
	case "confirmed":
		return []string{"completed"}
	default:
		return []string{"completed"}
	}
}

func (r *PostgresRepository) getFulfillmentByID(ctx context.Context, tenantID, regionID, id string) (Fulfillment, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, order_id, status, COALESCE(tracking_number,''), updated_at
FROM fulfillments
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, id, tenantID, regionID)
	return scanFulfillment(row)
}

func mapOrderRow(row dbsqlc.Order) orders.Order {
	return orders.Order{
		ID:         row.ID,
		TenantID:   row.TenantID,
		RegionID:   row.RegionID,
		CustomerID: row.CustomerID,
		Status:     row.Status,
		TotalCents: row.TotalCents,
		Currency:   row.Currency,
		CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (r *PostgresRepository) List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, order_id, status, COALESCE(tracking_number,''), updated_at
FROM fulfillments
WHERE tenant_id = $1 AND region_id = $2 AND ($3::text = '' OR order_id = $3)
ORDER BY created_at DESC
`, tenantID, regionID, orderID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make([]Fulfillment, 0)
	for rows.Next() {
		var f Fulfillment
		var updatedAt time.Time
		if err := rows.Scan(&f.ID, &f.TenantID, &f.RegionID, &f.OrderID, &f.Status, &f.TrackingNumber, &updatedAt); err != nil {
			return nil, err
		}
		f.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, f)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFulfillment(row scanner) (Fulfillment, error) {
	var out Fulfillment
	var updatedAt time.Time
	if err := row.Scan(&out.ID, &out.TenantID, &out.RegionID, &out.OrderID, &out.Status, &out.TrackingNumber, &updatedAt); err != nil {
		return Fulfillment{}, err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return out, nil
}
