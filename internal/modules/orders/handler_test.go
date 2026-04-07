package orders

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

type orderFakeRepo struct {
	orders map[string]Order
}

func (f *orderFakeRepo) Insert(_ context.Context, order Order) (Order, error) {
	if f.orders == nil {
		f.orders = map[string]Order{}
	}
	f.orders[order.ID] = order
	return order, nil
}

func (f *orderFakeRepo) UpdateStatus(_ context.Context, tenantID, orderID, status string) (Order, error) {
	order := f.orders[orderID]
	order.Status = status
	order.TenantID = tenantID
	f.orders[orderID] = order
	return order, nil
}

func (f *orderFakeRepo) List(_ context.Context, tenantID, _ string) ([]Order, error) {
	out := []Order{}
	for _, order := range f.orders {
		if order.TenantID == tenantID {
			out = append(out, order)
		}
	}
	return out, nil
}

func TestOrdersListIsTenantScoped(t *testing.T) {
	repo := &orderFakeRepo{
		orders: map[string]Order{
			"o1": {ID: "o1", TenantID: "tenant_a", Status: "created"},
			"o2": {ID: "o2", TenantID: "tenant_b", Status: "created"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/orders/", nil)
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
