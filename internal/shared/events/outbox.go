package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"rewrite/internal/shared/utils"
)

type OutboxStore struct {
	db *sql.DB
}

func NewOutboxStore(db *sql.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

func (s *OutboxStore) Enqueue(ctx context.Context, tenantID, regionID, eventName, aggregateType, aggregateID string, payload any) {
	body, _ := json.Marshal(payload)
	_, _ = s.db.ExecContext(ctx, `
INSERT INTO event_outbox (id, tenant_id, region_id, event_name, aggregate_type, aggregate_id, payload, status, available_at, attempts, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,'pending',NOW(),0,NOW(),NOW())
`, utils.NewID("evt"), tenantID, regionID, eventName, aggregateType, aggregateID, string(body))
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	RegionID  string
	EventName string
	Payload   string
}

func (s *OutboxStore) DequeuePending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, event_name, payload::text
FROM event_outbox WHERE status = 'pending' AND available_at <= NOW()
ORDER BY created_at ASC LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OutboxEvent{}
	for rows.Next() {
		var item OutboxEvent
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.EventName, &item.Payload); err == nil {
			out = append(out, item)
		}
	}
	return out, rows.Err()
}

func (s *OutboxStore) MarkDone(ctx context.Context, id string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE event_outbox SET status='done', updated_at=NOW() WHERE id=$1`, id)
}

func (s *OutboxStore) MarkRetry(ctx context.Context, id string, attempts int64) {
	_, _ = s.db.ExecContext(ctx, `UPDATE event_outbox SET status='pending', attempts=$2, available_at=$3, updated_at=NOW() WHERE id=$1`, id, attempts, time.Now().UTC().Add(time.Duration(attempts+1)*time.Second))
}
