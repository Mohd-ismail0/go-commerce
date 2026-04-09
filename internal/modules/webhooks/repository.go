package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"rewrite/internal/shared/utils"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, in Subscription) (Subscription, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO webhook_subscriptions (id, tenant_id, region_id, app_id, event_name, endpoint_url, secret, is_active, created_at, updated_at)
VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  region_id = EXCLUDED.region_id,
  app_id = EXCLUDED.app_id,
  event_name = EXCLUDED.event_name,
  endpoint_url = EXCLUDED.endpoint_url,
  secret = EXCLUDED.secret,
  is_active = EXCLUDED.is_active,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, COALESCE(app_id,''), event_name, endpoint_url, COALESCE(secret,''), is_active, updated_at
`, in.ID, in.TenantID, in.RegionID, in.AppID, in.EventName, in.EndpointURL, in.Secret, in.IsActive)
	return scanSubscription(row)
}

func (r *Repository) GetByID(ctx context.Context, tenantID, regionID, id string) (Subscription, bool, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, COALESCE(app_id,''), event_name, endpoint_url, COALESCE(secret,''), is_active, updated_at
FROM webhook_subscriptions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, id, tenantID, regionID)
	item, err := scanSubscription(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, err
	}
	return item, true, nil
}

func (r *Repository) AppExists(ctx context.Context, tenantID, regionID, appID string) (bool, error) {
	var found bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM apps
  WHERE tenant_id = $1 AND region_id = $2 AND id = $3
)
`, tenantID, regionID, appID).Scan(&found)
	return found, err
}

func (r *Repository) ListDeliveries(ctx context.Context, tenantID, regionID, status, eventName string, limit int) ([]Delivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT d.outbox_id, d.subscription_id, d.status, COALESCE(d.response_status, 0), COALESCE(d.response_body, ''), d.attempts, d.next_retry_at, d.updated_at
FROM webhook_deliveries d
JOIN event_outbox o ON o.id = d.outbox_id AND o.tenant_id = d.tenant_id AND o.region_id = d.region_id
WHERE d.tenant_id = $1
  AND d.region_id = $2
  AND ($3::text = '' OR d.status = $3)
  AND ($4::text = '' OR o.event_name = $4)
ORDER BY d.updated_at DESC
LIMIT $5
`, tenantID, regionID, status, eventName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var item Delivery
		var nextRetryAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(
			&item.OutboxID, &item.SubscriptionID, &item.Status, &item.ResponseStatus, &item.ResponseBody, &item.Attempts, &nextRetryAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if nextRetryAt.Valid {
			item.NextRetryAt = nextRetryAt.Time.UTC().Format(time.RFC3339Nano)
		}
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) RetryDeadOutbox(ctx context.Context, tenantID, regionID, outboxID, reason, requestedBy string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
UPDATE event_outbox
SET status = 'pending',
    available_at = NOW(),
    attempts = 0,
    updated_at = NOW()
WHERE id = $1
  AND tenant_id = $2
  AND region_id = $3
  AND status = 'dead'
`, outboxID, tenantID, regionID)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO webhook_replay_audit (id, tenant_id, region_id, outbox_id, reason, requested_by, created_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NOW())
`, utils.NewID("wra"), tenantID, regionID, outboxID, reason, requestedBy)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, COALESCE(app_id,''), event_name, endpoint_url, COALESCE(secret,''), is_active, updated_at
FROM webhook_subscriptions
WHERE tenant_id = $1 AND region_id = $2 AND ($3::bool = FALSE OR is_active = TRUE)
ORDER BY created_at DESC
`, tenantID, regionID, onlyActive)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]Subscription, 0)
	for rows.Next() {
		var item Subscription
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.AppID, &item.EventName, &item.EndpointURL, &item.Secret, &item.IsActive, &updatedAt); err != nil {
			return nil, err
		}
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, item)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanSubscription(row scanner) (Subscription, error) {
	var out Subscription
	var updatedAt time.Time
	if err := row.Scan(&out.ID, &out.TenantID, &out.RegionID, &out.AppID, &out.EventName, &out.EndpointURL, &out.Secret, &out.IsActive, &updatedAt); err != nil {
		return Subscription{}, err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return out, nil
}
