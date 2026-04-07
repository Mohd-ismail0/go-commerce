package orders

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/shared/events"
)

type fakeRepo struct {
	orders map[string]Order
}

func (f *fakeRepo) Insert(_ context.Context, order Order, _ string) (Order, error) {
	if f.orders == nil {
		f.orders = map[string]Order{}
	}
	f.orders[order.ID] = order
	return order, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	order := f.orders[input.ID]
	order.TenantID = tenantID
	order.Status = input.Status
	f.orders[input.ID] = order
	return order, nil
}

func (f *fakeRepo) List(_ context.Context, tenantID, _ string, _ *time.Time, _ int32) ([]Order, error) {
	var out []Order
	for _, order := range f.orders {
		if order.TenantID == tenantID {
			out = append(out, order)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetByID(_ context.Context, tenantID, orderID string) (Order, error) {
	order := f.orders[orderID]
	order.TenantID = tenantID
	return order, nil
}

func TestUpdateStatusRejectsInvalidState(t *testing.T) {
	repo := &fakeRepo{orders: map[string]Order{
		"ord_1": {ID: "ord_1", TenantID: "tenant_a", Status: "created"},
	}}
	svc := NewService(repo, events.NewBus())
	_, err := svc.UpdateStatus(context.Background(), "tenant_a", StatusUpdateInput{
		ID:                "ord_1",
		Status:            "invalid",
		ExpectedUpdatedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestUpdateStatusAllowsCreatedToConfirmed(t *testing.T) {
	repo := &fakeRepo{orders: map[string]Order{
		"ord_2": {ID: "ord_2", TenantID: "tenant_a", Status: "created"},
	}}
	svc := NewService(repo, events.NewBus())
	updated, err := svc.UpdateStatus(context.Background(), "tenant_a", StatusUpdateInput{
		ID:                "ord_2",
		Status:            "confirmed",
		ExpectedUpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != "confirmed" {
		t.Fatalf("expected status confirmed, got %s", updated.Status)
	}
}
