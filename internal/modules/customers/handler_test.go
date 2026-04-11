package customers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/middleware"
)

func TestCustomersPostRequiresIdempotencyKey(t *testing.T) {
	h := NewHandler(NewService(&stubRepo{}))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBufferString(`{"email":"a@b.co","name":"N"}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestCustomerPostAddressRequiresIdempotencyKey(t *testing.T) {
	h := NewHandler(NewService(&stubRepo{}))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	body := `{"first_name":"A","last_name":"B","street_line_1":"1","city":"C","postal_code":"1","country_code":"US"}`
	req := httptest.NewRequest(http.MethodPost, "/customers/c1/addresses", bytes.NewBufferString(body))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}
