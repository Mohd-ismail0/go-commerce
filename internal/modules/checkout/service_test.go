package checkout

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/shared/events"
)

type fakeRepo struct {
	completed bool
}

func (f *fakeRepo) CreateSession(_ context.Context, in Session) (Session, error) {
	return in, nil
}

func (f *fakeRepo) UpsertLine(_ context.Context, _, _ string, line Line) (Line, error) {
	return line, nil
}

func (f *fakeRepo) Recalculate(_ context.Context, _, _, checkoutID string) (Session, error) {
	return Session{ID: checkoutID, TotalCents: 1200}, nil
}

func (f *fakeRepo) Complete(_ context.Context, tenantID, regionID, checkoutID, orderID string) (OrderCreatedPayload, error) {
	f.completed = true
	return OrderCreatedPayload{
		ID:         orderID,
		TenantID:   tenantID,
		RegionID:   regionID,
		CheckoutID: checkoutID,
		CustomerID: "cus_1",
		Status:     "created",
		TotalCents: 1200,
		Currency:   "USD",
	}, nil
}

func TestCompletePublishesOrderCreatedAndReturnsOrderID(t *testing.T) {
	repo := &fakeRepo{}
	bus := events.NewBus()
	done := make(chan struct{}, 1)
	bus.Subscribe(events.EventOrderCreated, func(_ context.Context, payload any) {
		evt, ok := payload.(OrderCreatedPayload)
		if !ok {
			t.Fatalf("unexpected payload type")
		}
		if evt.CheckoutID != "chk_1" {
			t.Fatalf("expected checkout id chk_1, got %s", evt.CheckoutID)
		}
		done <- struct{}{}
	})
	svc := NewService(repo, bus)
	result, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID == "" {
		t.Fatalf("expected order id")
	}
	if !repo.completed {
		t.Fatalf("expected repository complete to be called")
	}
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected order.created event to be published")
	}
}
