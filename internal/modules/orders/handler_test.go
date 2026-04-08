package orders

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

type orderFakeRepo struct {
	orders map[string]Order
}

func TestOrdersCreateRequiresIdempotencyKey(t *testing.T) {
	repo := &orderFakeRepo{orders: map[string]Order{}}
	h := NewHandler(NewService(repo, events.NewBus(), nil))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/orders/", bytes.NewBufferString(`{"customer_id":"c1","currency":"USD","total_cents":1000}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func (f *orderFakeRepo) Insert(_ context.Context, order Order, _ string) (Order, error) {
	if f.orders == nil {
		f.orders = map[string]Order{}
	}
	f.orders[order.ID] = order
	return order, nil
}

func (f *orderFakeRepo) InsertWithVoucher(_ context.Context, order Order, _ string, _ string) (Order, error) {
	return f.Insert(context.Background(), order, "")
}

func (f *orderFakeRepo) UpdateStatus(_ context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	order := f.orders[input.ID]
	order.Status = input.Status
	order.TenantID = tenantID
	f.orders[input.ID] = order
	return order, nil
}

func (f *orderFakeRepo) UpdateStatusAndRestock(_ context.Context, tenantID string, input StatusUpdateInput) (Order, error) {
	return f.UpdateStatus(context.Background(), tenantID, input)
}

func (f *orderFakeRepo) List(_ context.Context, tenantID, _ string, _ *time.Time, _ int32) ([]Order, error) {
	out := []Order{}
	for _, order := range f.orders {
		if order.TenantID == tenantID {
			out = append(out, order)
		}
	}
	return out, nil
}

func (f *orderFakeRepo) GetByID(_ context.Context, tenantID, orderID string) (Order, error) {
	order := f.orders[orderID]
	order.TenantID = tenantID
	return order, nil
}

func TestOrdersListIsTenantScoped(t *testing.T) {
	repo := &orderFakeRepo{
		orders: map[string]Order{
			"o1": {ID: "o1", TenantID: "tenant_a", Status: "created"},
			"o2": {ID: "o2", TenantID: "tenant_b", Status: "created"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus(), nil))
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
	if !strings.Contains(body, "tenant_a") || strings.Contains(body, "tenant_b") {
		t.Fatalf("expected response scoped to tenant_a, got body: %s", body)
	}
}
