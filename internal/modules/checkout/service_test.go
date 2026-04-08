package checkout

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/modules/pricing"
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

func (f *fakeRepo) UpdateSessionContext(_ context.Context, _, _, checkoutID string, in Session) (Session, error) {
	in.ID = checkoutID
	in.Currency = "USD"
	in.SubtotalCents = 1000
	in.ShippingCents = 200
	return in, nil
}

func (f *fakeRepo) Recalculate(_ context.Context, _, _, checkoutID string) (Session, error) {
	return Session{ID: checkoutID, Currency: "USD", SubtotalCents: 1000, ShippingCents: 200, TotalCents: 1200}, nil
}

func (f *fakeRepo) UpdatePricing(_ context.Context, _, _, checkoutID string, taxCents, totalCents int64) (Session, error) {
	return Session{ID: checkoutID, Currency: "USD", SubtotalCents: 1000, ShippingCents: 200, TaxCents: taxCents, TotalCents: totalCents}, nil
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

type fakeCalculator struct{}

func (f *fakeCalculator) Calculate(_ context.Context, in pricing.CalculationInput) pricing.CalculationResult {
	return pricing.CalculationResult{
		BaseAmountCents: in.BaseAmountCents,
		DiscountCents:   100,
		TaxCents:        50,
		TotalCents:      in.BaseAmountCents - 100 + 50,
	}
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
	svc := NewService(repo, bus, &fakeCalculator{})
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

func TestUpdateSessionContextRecalculatesTotals(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	updated, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SAVE10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.TaxCents == 0 || updated.TotalCents == 0 {
		t.Fatalf("expected recalculated totals, got %+v", updated)
	}
}
