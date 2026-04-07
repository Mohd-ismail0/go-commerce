package fulfillments

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrOrderNotFound = errors.New("order not found")

type Repository interface {
	Create(ctx context.Context, in Fulfillment) (Fulfillment, error)
	List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewRepository(conn *sql.DB) Repository {
	return &PostgresRepository{db: conn}
}

func (r *PostgresRepository) Create(ctx context.Context, in Fulfillment) (Fulfillment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Fulfillment{}, err
	}
	defer tx.Rollback()

	var found int
	if err = tx.QueryRowContext(ctx, `
SELECT 1 FROM orders WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, in.OrderID, in.TenantID, in.RegionID).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Fulfillment{}, ErrOrderNotFound
		}
		return Fulfillment{}, err
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO fulfillments (id, tenant_id, region_id, order_id, status, tracking_number, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NOW(),NOW())
RETURNING id, tenant_id, region_id, order_id, status, COALESCE(tracking_number,''), updated_at
`, in.ID, in.TenantID, in.RegionID, in.OrderID, in.Status, in.TrackingNumber)
	saved, err := scanFulfillment(row)
	if err != nil {
		return Fulfillment{}, err
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE orders SET status = 'completed', updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, in.OrderID, in.TenantID, in.RegionID); err != nil {
		return Fulfillment{}, err
	}
	if err = tx.Commit(); err != nil {
		return Fulfillment{}, err
	}
	return saved, nil
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
	defer rows.Close()

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
