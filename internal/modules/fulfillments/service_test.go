package fulfillments

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/modules/orders"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

func waitForOrderCompleted(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for order.completed event")
	}
}

type fakeRepository struct {
	created Fulfillment
	out     CreateResult
	err     error
}

func (f *fakeRepository) Create(_ context.Context, in Fulfillment, _ string) (CreateResult, error) {
	if f.err != nil {
		return CreateResult{}, f.err
	}
	f.created = in
	if f.out.FinalOrder.ID != "" || f.out.Fulfillment.ID != "" {
		return f.out, nil
	}
	fo := orders.Order{
		ID:        in.OrderID,
		TenantID:  in.TenantID,
		RegionID:  in.RegionID,
		Status:    "completed",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	return CreateResult{
		Fulfillment:           in,
		FinalOrder:            fo,
		FromIdempotencyReplay: false,
		EmitOrderCompleted:    true,
	}, nil
}

func (f *fakeRepository) List(_ context.Context, _, _, _ string) ([]Fulfillment, error) {
	return []Fulfillment{f.created}, nil
}

func TestCreateSetsDefaultStatusAndPersists(t *testing.T) {
	repo := &fakeRepository{}
	bus := events.NewBus()
	completed := 0
	ch := make(chan struct{}, 1)
	bus.Subscribe(events.EventOrderCompleted, func(_ context.Context, _ any) {
		completed++
		ch <- struct{}{}
	})
	svc := NewService(repo, bus)
	saved, err := svc.Create(context.Background(), Fulfillment{
		ID:       "ful_1",
		TenantID: "tenant_a",
		RegionID: "us",
		OrderID:  "ord_1",
	}, "idem-create-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOrderCompleted(t, ch)
	if saved.Status != "fulfilled" {
		t.Fatalf("expected status fulfilled, got %s", saved.Status)
	}
	if repo.created.OrderID != "ord_1" {
		t.Fatalf("expected order id ord_1, got %s", repo.created.OrderID)
	}
	if completed != 1 {
		t.Fatalf("expected one order.completed event, got %d", completed)
	}
}

func TestCreateSkipsEventWhenOrderAlreadyCompleted(t *testing.T) {
	repo := &fakeRepository{
		out: CreateResult{
			Fulfillment:           Fulfillment{ID: "ful_1", OrderID: "ord_1", TenantID: "tenant_a", RegionID: "us", Status: "fulfilled"},
			FinalOrder:            orders.Order{ID: "ord_1", TenantID: "tenant_a", RegionID: "us", Status: "completed"},
			FromIdempotencyReplay: false,
			EmitOrderCompleted:    false,
		},
	}
	bus := events.NewBus()
	ch := make(chan struct{}, 1)
	bus.Subscribe(events.EventOrderCompleted, func(_ context.Context, _ any) { ch <- struct{}{} })
	svc := NewService(repo, bus)
	_, err := svc.Create(context.Background(), Fulfillment{
		OrderID:  "ord_1",
		TenantID: "tenant_a",
		RegionID: "us",
	}, "idem-already-done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-ch:
		t.Fatal("did not expect order.completed when EmitOrderCompleted is false")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestCreateSkipsEventOnIdempotentReplay(t *testing.T) {
	repo := &fakeRepository{
		out: CreateResult{
			Fulfillment:           Fulfillment{ID: "ful_1", OrderID: "ord_1", TenantID: "tenant_a", RegionID: "us", Status: "fulfilled"},
			FinalOrder:            orders.Order{ID: "ord_1", TenantID: "tenant_a", RegionID: "us", Status: "completed"},
			FromIdempotencyReplay: true,
			EmitOrderCompleted:    false,
		},
	}
	bus := events.NewBus()
	completed := 0
	ch := make(chan struct{}, 1)
	bus.Subscribe(events.EventOrderCompleted, func(_ context.Context, _ any) {
		completed++
		ch <- struct{}{}
	})
	svc := NewService(repo, bus)
	_, err := svc.Create(context.Background(), Fulfillment{
		ID:       "ful_1",
		TenantID: "tenant_a",
		RegionID: "us",
		OrderID:  "ord_1",
	}, "idem-replay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-ch:
		t.Fatal("did not expect order.completed on idempotent replay")
	case <-time.After(200 * time.Millisecond):
	}
	if completed != 0 {
		t.Fatalf("expected no order.completed on idempotent replay, got %d", completed)
	}
}

func TestCreateRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepository{}, events.NewBus())
	_, err := svc.Create(context.Background(), Fulfillment{OrderID: "ord_1", TenantID: "t", RegionID: "r"}, "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(sharederrors.APIError); !ok {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestCreateMapsOrderNotFulfillableToConflict(t *testing.T) {
	repo := &fakeRepository{err: ErrOrderNotFulfillable}
	svc := NewService(repo, events.NewBus())
	_, err := svc.Create(context.Background(), Fulfillment{
		OrderID:  "ord_1",
		TenantID: "tenant_a",
		RegionID: "us",
	}, "idem-x")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestCreateMapsOptimisticLockToConflict(t *testing.T) {
	repo := &fakeRepository{err: orders.ErrOptimisticLockFailed}
	svc := NewService(repo, events.NewBus())
	_, err := svc.Create(context.Background(), Fulfillment{
		OrderID:  "ord_1",
		TenantID: "tenant_a",
		RegionID: "us",
	}, "idem-y")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}
