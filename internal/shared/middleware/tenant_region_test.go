package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantRegionRejectsInvalidHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := TenantRegion("public", "global")(next)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Tenant-ID", "INVALID TENANT")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}
