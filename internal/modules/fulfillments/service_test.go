package fulfillments

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/modules/orders"
)

type fakeRepository struct {
	created Fulfillment
}

func (f *fakeRepository) Create(_ context.Context, in Fulfillment) (Fulfillment, error) {
	f.created = in
	return in, nil
}

func (f *fakeRepository) List(_ context.Context, _, _, _ string) ([]Fulfillment, error) {
	return []Fulfillment{f.created}, nil
}

type fakeOrderUpdater struct {
	currentOrder orders.Order
	updates      []orders.StatusUpdateInput
}

func (f *fakeOrderUpdater) GetByID(_ context.Context, _, _ string) (orders.Order, error) {
	return f.currentOrder, nil
}

func (f *fakeOrderUpdater) UpdateStatus(_ context.Context, _ string, input orders.StatusUpdateInput) (orders.Order, error) {
	f.updates = append(f.updates, input)
	f.currentOrder.Status = input.Status
	f.currentOrder.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return f.currentOrder, nil
}

func TestCreateSetsDefaultStatusAndPersists(t *testing.T) {
	repo := &fakeRepository{}
	orderUpdater := &fakeOrderUpdater{currentOrder: orders.Order{
		ID:        "ord_1",
		TenantID:  "tenant_a",
		Status:    "created",
		UpdatedAt: "2026-04-08T10:00:00Z",
	}}
	svc := NewService(repo, orderUpdater)
	saved, err := svc.Create(context.Background(), Fulfillment{
		ID:       "ful_1",
		TenantID: "tenant_a",
		RegionID: "us",
		OrderID:  "ord_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.Status != "fulfilled" {
		t.Fatalf("expected status fulfilled, got %s", saved.Status)
	}
	if repo.created.OrderID != "ord_1" {
		t.Fatalf("expected order id ord_1, got %s", repo.created.OrderID)
	}
	if len(orderUpdater.updates) != 2 {
		t.Fatalf("expected two order transitions (confirm->complete), got %d", len(orderUpdater.updates))
	}
	if orderUpdater.updates[0].Status != "confirmed" || orderUpdater.updates[1].Status != "completed" {
		t.Fatalf("unexpected transition sequence: %#v", orderUpdater.updates)
	}
}
