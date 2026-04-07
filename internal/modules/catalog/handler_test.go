package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/middleware"
)

type catalogFakeRepo struct {
	items []Product
}

func (r *catalogFakeRepo) Upsert(_ context.Context, product Product) (Product, error) {
	r.items = append(r.items, product)
	return product, nil
}

func (r *catalogFakeRepo) List(_ context.Context, tenantID, _, _ string) ([]Product, error) {
	out := []Product{}
	for _, p := range r.items {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestProductsListIsTenantScoped(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{
			{ID: "p1", TenantID: "tenant_a", Name: "A"},
			{ID: "p2", TenantID: "tenant_b", Name: "B"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/products/", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !(strings.Contains(body, "tenant_a") && !strings.Contains(body, "tenant_b")) {
		t.Fatalf("expected response scoped to tenant_a, got body: %s", body)
	}
}
