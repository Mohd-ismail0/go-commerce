package apps

import (
	"context"
	"errors"
	"testing"
)

type stubRepo struct {
	saveFn    func(ctx context.Context, in App) (App, error)
	listFn    func(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error)
	getByIDFn func(ctx context.Context, tenantID, regionID, id string) (App, bool, error)
}

func (s *stubRepo) Save(ctx context.Context, in App) (App, error) {
	if s.saveFn != nil {
		return s.saveFn(ctx, in)
	}
	return in, nil
}

func (s *stubRepo) List(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, regionID, activeOnly)
	}
	return nil, nil
}

func (s *stubRepo) GetByID(ctx context.Context, tenantID, regionID, id string) (App, bool, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, tenantID, regionID, id)
	}
	return App{}, false, nil
}

func TestPatchPreservesAuthTokenWhenOmitted(t *testing.T) {
	repo := &stubRepo{
		getByIDFn: func(_ context.Context, _, _, _ string) (App, bool, error) {
			return App{ID: "app_1", TenantID: "t1", RegionID: "r1", Name: "A", IsActive: true, AuthToken: "tok_old"}, true, nil
		},
	}
	svc := NewService(repo)
	saved, err := svc.Save(context.Background(), App{
		ID:       "app_1",
		TenantID: "t1",
		RegionID: "r1",
		Name:     "A2",
		// AuthToken intentionally omitted.
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.AuthToken != "tok_old" {
		t.Fatalf("expected old auth token preserved, got %q", saved.AuthToken)
	}
	if saved.Name != "A2" {
		t.Fatalf("expected name updated, got %q", saved.Name)
	}
}

func TestCreateDuplicateNameMapsConflict(t *testing.T) {
	repo := &stubRepo{
		saveFn: func(_ context.Context, _ App) (App, error) {
			return App{}, errors.New("duplicate key value violates unique constraint ux_apps_tenant_region_name_ci")
		},
	}
	svc := NewService(repo)
	_, err := svc.Save(context.Background(), App{
		ID:       "app_2",
		TenantID: "t1",
		RegionID: "r1",
		Name:     "Payments",
		IsActive: true,
	}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSetActiveNotFound(t *testing.T) {
	repo := &stubRepo{
		getByIDFn: func(_ context.Context, _, _, _ string) (App, bool, error) {
			return App{}, false, nil
		},
	}
	svc := NewService(repo)
	_, err := svc.SetActive(context.Background(), "t1", "r1", "missing", true)
	if err == nil {
		t.Fatalf("expected error")
	}
}
