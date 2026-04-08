package events

import (
	"context"
	"fmt"
	"time"
)

type outboxWorkerStore interface {
	DequeuePending(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkDone(ctx context.Context, id string) error
	MarkRetry(ctx context.Context, id string, attempts int64, nextRetryAt time.Time) error
	MarkFailed(ctx context.Context, id string) error
	ListActiveWebhookSubscriptions(ctx context.Context, tenantID, regionID, eventName string) ([]WebhookSubscription, error)
	RecordDeliveryAttempt(ctx context.Context, item DeliveryAttempt) error
}

type webhookDeliverer interface {
	Deliver(ctx context.Context, event string, payload any, targets []WebhookDeliveryInput) []WebhookDeliveryResult
}

type Worker struct {
	store      outboxWorkerStore
	dispatcher webhookDeliverer
	now        func() time.Time
	maxRetries int64
}

func NewWorker(store *OutboxStore, dispatcher *WebhookDispatcher) *Worker {
	return &Worker{
		store:      store,
		dispatcher: dispatcher,
		now:        time.Now,
		maxRetries: 8,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *Worker) tick(ctx context.Context) {
	events, err := w.store.DequeuePending(ctx, 50)
	if err != nil {
		return
	}
	for _, item := range events {
		nextAttempts := item.Attempts + 1
		nextRetryAt := retryAt(w.now(), nextAttempts)
		targetSubs, subErr := w.store.ListActiveWebhookSubscriptions(ctx, item.TenantID, item.RegionID, item.EventName)
		if subErr != nil {
			_ = w.store.MarkRetry(ctx, item.ID, nextAttempts, nextRetryAt)
			continue
		}
		if len(targetSubs) == 0 {
			if err := w.store.MarkDone(ctx, item.ID); err != nil {
				_ = w.store.MarkRetry(ctx, item.ID, nextAttempts, nextRetryAt)
			}
			continue
		}
		targets := make([]WebhookDeliveryInput, 0, len(targetSubs))
		for _, sub := range targetSubs {
			targets = append(targets, WebhookDeliveryInput{
				SubscriptionID: sub.ID,
				URL:            sub.EndpointURL,
				Secret:         sub.Secret,
			})
		}
		results := w.dispatcher.Deliver(ctx, item.EventName, map[string]any{"raw": item.Payload}, targets)
		allSucceeded := true
		for _, result := range results {
			status := "failed"
			var responseStatus *int
			if result.StatusCode > 0 {
				responseStatus = &result.StatusCode
			}
			if result.Err == nil && result.StatusCode >= 200 && result.StatusCode < 300 {
				status = "done"
			} else {
				allSucceeded = false
			}
			responseBody := result.ResponseBody
			if result.Err != nil {
				responseBody = fmt.Sprintf("delivery error: %v", result.Err)
			}
			recordErr := w.store.RecordDeliveryAttempt(ctx, DeliveryAttempt{
				OutboxID:       item.ID,
				TenantID:       item.TenantID,
				RegionID:       item.RegionID,
				SubscriptionID: result.SubscriptionID,
				Status:         status,
				ResponseStatus: responseStatus,
				ResponseBody:   responseBody,
				NextRetryAt:    timePtrIf(status != "done", nextRetryAt),
			})
			if recordErr != nil {
				allSucceeded = false
			}
		}
		if allSucceeded {
			_ = w.store.MarkDone(ctx, item.ID)
			continue
		}
		if nextAttempts >= w.maxRetries {
			_ = w.store.MarkFailed(ctx, item.ID)
			continue
		}
		_ = w.store.MarkRetry(ctx, item.ID, nextAttempts, nextRetryAt)
	}
}

func retryAt(now time.Time, attempts int64) time.Time {
	seconds := int64(1) << minInt64(attempts, 6)
	return now.UTC().Add(time.Duration(seconds) * time.Second)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func timePtrIf(cond bool, t time.Time) *time.Time {
	if !cond {
		return nil
	}
	tt := t.UTC()
	return &tt
}
