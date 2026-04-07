package orders

import (
	"context"
	"testing"

	"rewrite/internal/shared/events"
)

type fakeRepo struct {
	orders map[string]Order
}

func (f *fakeRepo) Insert(_ context.Context, order Order) (Order, error) {
	if f.orders == nil {
		f.orders = map[string]Order{}
	}
	f.orders[order.ID] = order
	return order, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, tenantID, orderID, status string) (Order, error) {
	order := f.orders[orderID]
	order.TenantID = tenantID
	order.Status = status
	f.orders[orderID] = order
	return order, nil
}

func (f *fakeRepo) List(_ context.Context, tenantID, _ string) ([]Order, error) {
	var out []Order
	for _, order := range f.orders {
		if order.TenantID == tenantID {
			out = append(out, order)
		}
	}
	return out, nil
}

func TestUpdateStatusRejectsInvalidState(t *testing.T) {
	repo := &fakeRepo{orders: map[string]Order{
		"ord_1": {ID: "ord_1", TenantID: "tenant_a", Status: "created"},
	}}
	svc := NewService(repo, events.NewBus())
	_, err := svc.UpdateStatus(context.Background(), "tenant_a", "ord_1", "invalid")
	if err == nil {
		t.Fatalf("expected invalid status error")
	}
}
