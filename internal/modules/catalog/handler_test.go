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
	items              []Product
	variants           []ProductVariant
	categories         []Category
	collections        []Collection
	collectionProducts map[string]map[string]bool
	media              []ProductMedia
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

func (r *catalogFakeRepo) UpsertVariant(_ context.Context, variant ProductVariant) (ProductVariant, error) {
	r.variants = append(r.variants, variant)
	return variant, nil
}

func (r *catalogFakeRepo) ListVariants(_ context.Context, tenantID, _, productID string) ([]ProductVariant, error) {
	out := []ProductVariant{}
	for _, v := range r.variants {
		if v.TenantID == tenantID && v.ProductID == productID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) IsSKUTenantRegionAvailable(_ context.Context, tenantID, regionID, sku, variantID string) (bool, error) {
	for _, p := range r.items {
		if p.TenantID == tenantID && p.RegionID == regionID && p.SKU == sku {
			return false, nil
		}
	}
	for _, v := range r.variants {
		if v.TenantID == tenantID && v.RegionID == regionID && v.SKU == sku && v.ID != variantID {
			return false, nil
		}
	}
	return true, nil
}

func (r *catalogFakeRepo) InsertCategory(_ context.Context, category Category) (Category, error) {
	r.categories = append(r.categories, category)
	return category, nil
}

func (r *catalogFakeRepo) ListCategories(_ context.Context, tenantID, regionID string) ([]Category, error) {
	out := []Category{}
	for _, c := range r.categories {
		if c.TenantID == tenantID && c.RegionID == regionID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) InsertCollection(_ context.Context, collection Collection) (Collection, error) {
	r.collections = append(r.collections, collection)
	return collection, nil
}

func (r *catalogFakeRepo) ListCollections(_ context.Context, tenantID, regionID string) ([]Collection, error) {
	out := []Collection{}
	for _, c := range r.collections {
		if c.TenantID == tenantID && c.RegionID == regionID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) AssignProductToCollection(_ context.Context, _, _, collectionID, productID string) error {
	if r.collectionProducts == nil {
		r.collectionProducts = map[string]map[string]bool{}
	}
	if r.collectionProducts[collectionID] == nil {
		r.collectionProducts[collectionID] = map[string]bool{}
	}
	r.collectionProducts[collectionID][productID] = true
	return nil
}

func (r *catalogFakeRepo) InsertProductMedia(_ context.Context, media ProductMedia) (ProductMedia, error) {
	r.media = append(r.media, media)
	return media, nil
}

func (r *catalogFakeRepo) ListProductMedia(_ context.Context, tenantID, regionID, productID string) ([]ProductMedia, error) {
	out := []ProductMedia{}
	for _, item := range r.media {
		if item.TenantID == tenantID && item.RegionID == regionID && item.ProductID == productID {
			out = append(out, item)
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

func TestVariantCreateRejectsDuplicateSKUInTenantRegion(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{{ID: "p1", TenantID: "tenant_a", RegionID: "global", SKU: "SKU-DUP", Name: "A", Currency: "USD", PriceCents: 100}},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/products/p1/variants/", bytes.NewBufferString(`{"sku":"SKU-DUP","name":"Red","currency":"USD","price_cents":120}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}
}

func TestVariantCreateAllowsSameSKUAcrossTenant(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{{ID: "p1", TenantID: "tenant_b", RegionID: "global", SKU: "SKU-X", Name: "A", Currency: "USD", PriceCents: 100}},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/products/p2/variants/", bytes.NewBufferString(`{"sku":"SKU-X","name":"Blue","currency":"USD","price_cents":120}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
}

func TestAssignProductToCollectionEndpoint(t *testing.T) {
	repo := &catalogFakeRepo{}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/collections/c1/products/p1", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if repo.collectionProducts == nil || !repo.collectionProducts["c1"]["p1"] {
		t.Fatalf("expected product to be assigned to collection")
	}
}
