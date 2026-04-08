package events

import (
	"context"
	"testing"
	"time"
)

type fakeWorkerStore struct {
	events     []OutboxEvent
	subs       []WebhookSubscription
	recorded   []DeliveryAttempt
	doneIDs    []string
	failedIDs  []string
	retryCalls []struct {
		id        string
		attempts  int64
		nextRetry time.Time
	}
}

func (f *fakeWorkerStore) DequeuePending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	return f.events, nil
}

func (f *fakeWorkerStore) MarkDone(ctx context.Context, id string) error {
	f.doneIDs = append(f.doneIDs, id)
	return nil
}

func (f *fakeWorkerStore) MarkRetry(ctx context.Context, id string, attempts int64, nextRetryAt time.Time) error {
	f.retryCalls = append(f.retryCalls, struct {
		id        string
		attempts  int64
		nextRetry time.Time
	}{id: id, attempts: attempts, nextRetry: nextRetryAt})
	return nil
}

func (f *fakeWorkerStore) MarkFailed(ctx context.Context, id string) error {
	f.failedIDs = append(f.failedIDs, id)
	return nil
}

func (f *fakeWorkerStore) ListActiveWebhookSubscriptions(ctx context.Context, tenantID, regionID, eventName string) ([]WebhookSubscription, error) {
	return f.subs, nil
}

func (f *fakeWorkerStore) RecordDeliveryAttempt(ctx context.Context, item DeliveryAttempt) error {
	f.recorded = append(f.recorded, item)
	return nil
}

type fakeDeliverer struct {
	results []WebhookDeliveryResult
}

func (f *fakeDeliverer) Deliver(ctx context.Context, event string, payload any, targets []WebhookDeliveryInput) []WebhookDeliveryResult {
	return f.results
}

func TestWorkerMarksDoneOnlyOnSuccessfulDeliveries(t *testing.T) {
	base := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeWorkerStore{
		events: []OutboxEvent{{ID: "evt1", TenantID: "t1", RegionID: "r1", EventName: EventOrderCreated, Payload: `{"x":1}`, Attempts: 0}},
		subs:   []WebhookSubscription{{ID: "sub1", EventName: EventOrderCreated, EndpointURL: "http://example.test"}},
	}
	deliverer := &fakeDeliverer{
		results: []WebhookDeliveryResult{{SubscriptionID: "sub1", StatusCode: 200, ResponseBody: "ok"}},
	}
	worker := &Worker{store: store, dispatcher: deliverer, now: func() time.Time { return base }, maxRetries: 8}

	worker.tick(context.Background())

	if len(store.doneIDs) != 1 || store.doneIDs[0] != "evt1" {
		t.Fatalf("expected outbox to be marked done once, got %#v", store.doneIDs)
	}
	if len(store.retryCalls) != 0 {
		t.Fatalf("expected no retry calls on success, got %#v", store.retryCalls)
	}
	if len(store.recorded) != 1 || store.recorded[0].Status != "done" {
		t.Fatalf("expected one successful delivery record, got %#v", store.recorded)
	}
}

func TestWorkerRetriesAndRecordsAttemptOnFailure(t *testing.T) {
	base := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeWorkerStore{
		events: []OutboxEvent{{ID: "evt2", TenantID: "t1", RegionID: "r1", EventName: EventOrderCreated, Payload: `{"x":1}`, Attempts: 1}},
		subs:   []WebhookSubscription{{ID: "sub2", EventName: EventOrderCreated, EndpointURL: "http://example.test"}},
	}
	deliverer := &fakeDeliverer{
		results: []WebhookDeliveryResult{{SubscriptionID: "sub2", StatusCode: 500, ResponseBody: "boom"}},
	}
	worker := &Worker{store: store, dispatcher: deliverer, now: func() time.Time { return base }, maxRetries: 8}

	worker.tick(context.Background())

	if len(store.doneIDs) != 0 {
		t.Fatalf("expected no done marks on failure, got %#v", store.doneIDs)
	}
	if len(store.retryCalls) != 1 {
		t.Fatalf("expected exactly one retry call, got %#v", store.retryCalls)
	}
	if store.retryCalls[0].attempts != 2 {
		t.Fatalf("expected attempts increment to 2, got %d", store.retryCalls[0].attempts)
	}
	expectedRetry := base.Add(4 * time.Second)
	if !store.retryCalls[0].nextRetry.Equal(expectedRetry) {
		t.Fatalf("expected retry at %s, got %s", expectedRetry, store.retryCalls[0].nextRetry)
	}
	if len(store.recorded) != 1 || store.recorded[0].Status != "failed" || store.recorded[0].NextRetryAt == nil {
		t.Fatalf("expected one failed delivery record with retry time, got %#v", store.recorded)
	}
}
