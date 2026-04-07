package fulfillments

import (
	"context"
	"testing"
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

func TestCreateSetsDefaultStatusAndPersists(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
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
}
