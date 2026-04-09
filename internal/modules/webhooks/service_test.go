package webhooks

import (
	"context"
	"errors"
	"testing"
)

type stubRepo struct {
	saveFn           func(ctx context.Context, in Subscription) (Subscription, error)
	listFn           func(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error)
	getByIDFn        func(ctx context.Context, tenantID, regionID, id string) (Subscription, bool, error)
	appExists        func(ctx context.Context, tenantID, regionID, appID string) (bool, error)
	listDeliveriesFn func(ctx context.Context, tenantID, regionID, status, eventName string, limit int) ([]Delivery, error)
	retryDeadFn      func(ctx context.Context, tenantID, regionID, outboxID string) (bool, error)
}

func (s *stubRepo) Save(ctx context.Context, in Subscription) (Subscription, error) {
	if s.saveFn != nil {
		return s.saveFn(ctx, in)
	}
	return in, nil
}

func (s *stubRepo) List(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, regionID, onlyActive)
	}
	return nil, nil
}

func (s *stubRepo) GetByID(ctx context.Context, tenantID, regionID, id string) (Subscription, bool, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, tenantID, regionID, id)
	}
	return Subscription{}, false, nil
}

func (s *stubRepo) AppExists(ctx context.Context, tenantID, regionID, appID string) (bool, error) {
	if s.appExists != nil {
		return s.appExists(ctx, tenantID, regionID, appID)
	}
	return true, nil
}

func (s *stubRepo) ListDeliveries(ctx context.Context, tenantID, regionID, status, eventName string, limit int) ([]Delivery, error) {
	if s.listDeliveriesFn != nil {
		return s.listDeliveriesFn(ctx, tenantID, regionID, status, eventName, limit)
	}
	return nil, nil
}

func (s *stubRepo) RetryDeadOutbox(ctx context.Context, tenantID, regionID, outboxID string) (bool, error) {
	if s.retryDeadFn != nil {
		return s.retryDeadFn(ctx, tenantID, regionID, outboxID)
	}
	return false, nil
}

func TestSaveRejectsUnsupportedEvent(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Save(context.Background(), Subscription{
		ID:          "whs_1",
		TenantID:    "t1",
		RegionID:    "r1",
		EventName:   "order.cancelled",
		EndpointURL: "https://example.test/hook",
		IsActive:    true,
	}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSaveRejectsMissingApp(t *testing.T) {
	svc := NewService(&stubRepo{
		appExists: func(context.Context, string, string, string) (bool, error) { return false, nil },
	})
	_, err := svc.Save(context.Background(), Subscription{
		ID:          "whs_2",
		TenantID:    "t1",
		RegionID:    "r1",
		AppID:       "missing",
		EventName:   "order.created",
		EndpointURL: "https://example.test/hook",
		IsActive:    true,
	}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestPatchPreservesSecretWhenOmitted(t *testing.T) {
	svc := NewService(&stubRepo{
		getByIDFn: func(context.Context, string, string, string) (Subscription, bool, error) {
			return Subscription{
				ID:          "whs_3",
				TenantID:    "t1",
				RegionID:    "r1",
				EventName:   "order.created",
				EndpointURL: "https://example.test/hook",
				Secret:      "oldsecret",
				IsActive:    true,
			}, true, nil
		},
	})
	saved, err := svc.Save(context.Background(), Subscription{
		ID:          "whs_3",
		TenantID:    "t1",
		RegionID:    "r1",
		EndpointURL: "https://example.test/new",
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.Secret != "oldsecret" {
		t.Fatalf("expected secret preserved, got %q", saved.Secret)
	}
}

func TestSaveMapsDuplicateConflict(t *testing.T) {
	svc := NewService(&stubRepo{
		saveFn: func(context.Context, Subscription) (Subscription, error) {
			return Subscription{}, errors.New("duplicate key value violates unique constraint ux_webhook_subscriptions_tenant_region_event_endpoint_app")
		},
	})
	_, err := svc.Save(context.Background(), Subscription{
		ID:          "whs_4",
		TenantID:    "t1",
		RegionID:    "r1",
		EventName:   "order.created",
		EndpointURL: "https://example.test/hook",
		IsActive:    true,
	}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRetryDeadOutboxNotFound(t *testing.T) {
	svc := NewService(&stubRepo{
		retryDeadFn: func(context.Context, string, string, string) (bool, error) { return false, nil },
	})
	err := svc.RetryDeadOutbox(context.Background(), "t1", "r1", "evt_missing")
	if err == nil {
		t.Fatalf("expected error")
	}
}
