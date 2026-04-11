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
