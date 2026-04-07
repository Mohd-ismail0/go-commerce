package catalog

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/middleware"
)

type catalogFakeRepo struct {
	items []Product
}

func TestProductsCreateRequiresIdempotencyKey(t *testing.T) {
	repo := &catalogFakeRepo{}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/products/", bytes.NewBufferString(`{"sku":"SKU-1","name":"A","currency":"USD","price_cents":100}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func (r *catalogFakeRepo) Upsert(_ context.Context, product Product, _ string) (Product, error) {
	r.items = append(r.items, product)
	return product, nil
}

func (r *catalogFakeRepo) List(_ context.Context, tenantID, _, _ string, _ *time.Time, _ int32) ([]Product, error) {
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
