package events

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"rewrite/internal/shared/utils"
)

type OutboxStore struct {
	db *sql.DB
}

func NewOutboxStore(db *sql.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

func (s *OutboxStore) Enqueue(ctx context.Context, tenantID, regionID, eventName, aggregateType, aggregateID string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO event_outbox (id, tenant_id, region_id, event_name, aggregate_type, aggregate_id, payload, status, available_at, attempts, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,'pending',NOW(),0,NOW(),NOW())
`, utils.NewID("evt"), tenantID, regionID, eventName, aggregateType, aggregateID, string(body))
	return err
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	RegionID  string
	EventName string
	Payload   string
	Attempts  int64
}

func (s *OutboxStore) DequeuePending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
WITH picked AS (
  SELECT id
  FROM event_outbox
  WHERE status = 'pending' AND available_at <= NOW()
  ORDER BY created_at ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE event_outbox e
SET status='processing', updated_at=NOW()
FROM picked
WHERE e.id = picked.id
RETURNING e.id, e.tenant_id, e.region_id, e.event_name, e.payload::text, e.attempts
`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []OutboxEvent{}
	for rows.Next() {
		var item OutboxEvent
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.EventName, &item.Payload, &item.Attempts); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *OutboxStore) MarkDone(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET status='done', updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (s *OutboxStore) MarkRetry(ctx context.Context, id string, attempts int64, nextRetryAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET status='pending', attempts=$2, available_at=$3, updated_at=NOW() WHERE id=$1`, id, attempts, nextRetryAt.UTC())
	return err
}

func (s *OutboxStore) MarkFailed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET status='dead', updated_at=NOW() WHERE id=$1`, id)
	return err
}

type DeliveryAttempt struct {
	OutboxID       string
	TenantID       string
	RegionID       string
	SubscriptionID string
	Status         string
	ResponseStatus *int
	ResponseBody   string
	NextRetryAt    *time.Time
}

type WebhookSubscription struct {
	ID          string
	EventName   string
	EndpointURL string
	Secret      string
}

func (s *OutboxStore) ListActiveWebhookSubscriptions(ctx context.Context, tenantID, regionID, eventName string) ([]WebhookSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, event_name, endpoint_url, COALESCE(secret,'')
FROM webhook_subscriptions
WHERE tenant_id=$1 AND region_id=$2 AND event_name=$3 AND is_active=TRUE
`, tenantID, regionID, eventName)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []WebhookSubscription{}
	for rows.Next() {
		var sub WebhookSubscription
		if err := rows.Scan(&sub.ID, &sub.EventName, &sub.EndpointURL, &sub.Secret); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func SignWebhookPayload(secret string, payload []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *OutboxStore) RecordDeliveryAttempt(ctx context.Context, item DeliveryAttempt) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO webhook_deliveries (id, tenant_id, region_id, outbox_id, subscription_id, status, response_status, response_body, attempts, next_retry_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9,NOW(),NOW())
ON CONFLICT (outbox_id, subscription_id) DO UPDATE SET
  status = EXCLUDED.status,
  response_status = EXCLUDED.response_status,
  response_body = EXCLUDED.response_body,
  attempts = webhook_deliveries.attempts + 1,
  next_retry_at = EXCLUDED.next_retry_at,
  updated_at = NOW()
`, utils.NewID("whd"), item.TenantID, item.RegionID, item.OutboxID, item.SubscriptionID, item.Status, item.ResponseStatus, item.ResponseBody, item.NextRetryAt)
	if err != nil {
		return fmt.Errorf("record webhook delivery attempt: %w", err)
	}
	return nil
}
