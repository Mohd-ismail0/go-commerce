package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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
	items                  []Product
	productTranslations    map[string]map[string]map[string]string
	categoryTranslations   map[string]map[string]map[string]string
	collectionTranslations map[string]map[string]map[string]string
	variants               []ProductVariant
	categories             []Category
	collections            []Collection
	collectionProducts     map[string]map[string]bool
	media                  []ProductMedia
	assignErr              error
	productChannelFilter   map[string][]string
	variantChannelFilter   map[string][]string
	productTypes           []ProductType
	catalogAttrs           []CatalogAttribute
	typeAttrLinks          map[string]map[string]linkStub
	productAttrVals        map[string][]AttributeValuePair
	variantAttrVals        map[string][]AttributeValuePair
}

type linkStub struct {
	sortOrder   int
	variantOnly bool
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

func (r *catalogFakeRepo) ListProductsByChannel(_ context.Context, tenantID, _, channelID, _ string, onlyPublished bool, _ *time.Time, _ int32) ([]Product, error) {
	out := []Product{}
	for _, p := range r.items {
		if p.TenantID != tenantID {
			continue
		}
		if channels, ok := r.productChannelFilter[p.ID]; ok {
			match := false
			for _, c := range channels {
				if c == channelID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if onlyPublished && strings.HasPrefix(p.Name, "UNPUBLISHED:") {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *catalogFakeRepo) ListProductTranslations(_ context.Context, _, _ string, productIDs []string, languageCode string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if r.productTranslations == nil || languageCode == "" {
		return out, nil
	}
	for _, id := range productIDs {
		if byLang, ok := r.productTranslations[id]; ok {
			if fields, ok := byLang[languageCode]; ok {
				out[id] = fields
			}
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) ListCategoryTranslations(_ context.Context, _, _ string, categoryIDs []string, languageCode string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if r.categoryTranslations == nil || languageCode == "" {
		return out, nil
	}
	for _, id := range categoryIDs {
		if byLang, ok := r.categoryTranslations[id]; ok {
			if fields, ok := byLang[languageCode]; ok {
				out[id] = fields
			}
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) ListCollectionTranslations(_ context.Context, _, _ string, collectionIDs []string, languageCode string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if r.collectionTranslations == nil || languageCode == "" {
		return out, nil
	}
	for _, id := range collectionIDs {
		if byLang, ok := r.collectionTranslations[id]; ok {
			if fields, ok := byLang[languageCode]; ok {
				out[id] = fields
			}
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) IsProductSlugAvailable(_ context.Context, tenantID, regionID, slug, productID string) (bool, error) {
	for _, p := range r.items {
		if p.TenantID == tenantID && p.RegionID == regionID && p.Slug == slug && p.ID != productID {
			return false, nil
		}
	}
	return true, nil
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

func (r *catalogFakeRepo) ListVariantsByChannel(_ context.Context, tenantID, _, productID, channelID string, onlyPublished bool) ([]ProductVariant, error) {
	out := []ProductVariant{}
	for _, v := range r.variants {
		if v.TenantID != tenantID || v.ProductID != productID {
			continue
		}
		if channels, ok := r.variantChannelFilter[v.ID]; ok {
			match := false
			for _, c := range channels {
				if c == channelID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if onlyPublished && strings.HasPrefix(v.Name, "UNPUBLISHED:") {
			continue
		}
		out = append(out, v)
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
	if r.assignErr != nil {
		return r.assignErr
	}
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

func (r *catalogFakeRepo) InsertProductType(_ context.Context, pt ProductType) (ProductType, error) {
	r.productTypes = append(r.productTypes, pt)
	return pt, nil
}

func (r *catalogFakeRepo) ListProductTypes(_ context.Context, tenantID, regionID string) ([]ProductType, error) {
	out := []ProductType{}
	for _, pt := range r.productTypes {
		if pt.TenantID == tenantID && pt.RegionID == regionID {
			out = append(out, pt)
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) GetProductType(_ context.Context, tenantID, regionID, id string) (ProductType, error) {
	for _, pt := range r.productTypes {
		if pt.ID == id && pt.TenantID == tenantID && pt.RegionID == regionID {
			return pt, nil
		}
	}
	return ProductType{}, sql.ErrNoRows
}

func (r *catalogFakeRepo) ListProductTypeAttributeDefs(_ context.Context, tenantID, regionID, productTypeID string) ([]ProductTypeAttributeDef, error) {
	if r.typeAttrLinks == nil || r.typeAttrLinks[productTypeID] == nil {
		return nil, nil
	}
	out := []ProductTypeAttributeDef{}
	for attrID, st := range r.typeAttrLinks[productTypeID] {
		var ca CatalogAttribute
		for _, a := range r.catalogAttrs {
			if a.ID == attrID && a.TenantID == tenantID && a.RegionID == regionID {
				ca = a
				break
			}
		}
		out = append(out, ProductTypeAttributeDef{
			AttributeID:   attrID,
			Name:          ca.Name,
			Slug:          ca.Slug,
			InputType:     ca.InputType,
			Unit:          ca.Unit,
			SortOrder:     st.sortOrder,
			VariantOnly:   st.variantOnly,
			AllowedValues: ca.AllowedValues,
		})
	}
	return out, nil
}

func (r *catalogFakeRepo) InsertCatalogAttribute(_ context.Context, a CatalogAttribute) (CatalogAttribute, error) {
	r.catalogAttrs = append(r.catalogAttrs, a)
	return a, nil
}

func (r *catalogFakeRepo) ListCatalogAttributes(_ context.Context, tenantID, regionID string) ([]CatalogAttribute, error) {
	out := []CatalogAttribute{}
	for _, a := range r.catalogAttrs {
		if a.TenantID == tenantID && a.RegionID == regionID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) GetCatalogAttribute(_ context.Context, tenantID, regionID, id string) (CatalogAttribute, error) {
	for _, a := range r.catalogAttrs {
		if a.ID == id && a.TenantID == tenantID && a.RegionID == regionID {
			return a, nil
		}
	}
	return CatalogAttribute{}, sql.ErrNoRows
}

func (r *catalogFakeRepo) LinkAttributeToProductType(_ context.Context, productTypeID string, in LinkAttributeToTypeInput) error {
	if r.typeAttrLinks == nil {
		r.typeAttrLinks = map[string]map[string]linkStub{}
	}
	if r.typeAttrLinks[productTypeID] == nil {
		r.typeAttrLinks[productTypeID] = map[string]linkStub{}
	}
	r.typeAttrLinks[productTypeID][in.AttributeID] = linkStub{sortOrder: in.SortOrder, variantOnly: in.VariantOnly}
	return nil
}

func (r *catalogFakeRepo) UnlinkAttributeFromProductType(_ context.Context, tenantID, regionID, productTypeID, attributeID string) error {
	found := false
	for _, pt := range r.productTypes {
		if pt.ID == productTypeID && pt.TenantID == tenantID && pt.RegionID == regionID {
			found = true
			break
		}
	}
	if !found {
		return sql.ErrNoRows
	}
	if r.typeAttrLinks != nil && r.typeAttrLinks[productTypeID] != nil {
		delete(r.typeAttrLinks[productTypeID], attributeID)
	}
	return nil
}

func (r *catalogFakeRepo) ListProductAttributeValues(_ context.Context, productID string) ([]AttributeValuePair, error) {
	if r.productAttrVals == nil {
		return nil, nil
	}
	return r.productAttrVals[productID], nil
}

func (r *catalogFakeRepo) ListVariantAttributeValues(_ context.Context, variantID string) ([]AttributeValuePair, error) {
	if r.variantAttrVals == nil {
		return nil, nil
	}
	return r.variantAttrVals[variantID], nil
}

func (r *catalogFakeRepo) ListProductAttributeValuesForProducts(_ context.Context, productIDs []string) (map[string][]AttributeValuePair, error) {
	out := map[string][]AttributeValuePair{}
	if r.productAttrVals == nil {
		return out, nil
	}
	for _, id := range productIDs {
		if v, ok := r.productAttrVals[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (r *catalogFakeRepo) SetProductAttributeValues(_ context.Context, productID string, pairs []AttributeValuePair) error {
	if r.productAttrVals == nil {
		r.productAttrVals = map[string][]AttributeValuePair{}
	}
	if len(pairs) == 0 {
		return nil
	}
	r.productAttrVals[productID] = append([]AttributeValuePair(nil), pairs...)
	return nil
}

func (r *catalogFakeRepo) SetVariantAttributeValues(_ context.Context, variantID string, pairs []AttributeValuePair) error {
	if r.variantAttrVals == nil {
		r.variantAttrVals = map[string][]AttributeValuePair{}
	}
	if len(pairs) == 0 {
		return nil
	}
	r.variantAttrVals[variantID] = append([]AttributeValuePair(nil), pairs...)
	return nil
}

func (r *catalogFakeRepo) GetProductRegionAndType(_ context.Context, tenantID, productID string) (regionID string, productTypeID string, hasType bool, err error) {
	for _, p := range r.items {
		if p.ID == productID && p.TenantID == tenantID {
			if strings.TrimSpace(p.ProductTypeID) != "" {
				return p.RegionID, p.ProductTypeID, true, nil
			}
			return p.RegionID, "", false, nil
		}
	}
	return "", "", false, sql.ErrNoRows
}

func (r *catalogFakeRepo) GetVariantProductRegion(_ context.Context, tenantID, variantID string) (productID, regionID string, err error) {
	for _, v := range r.variants {
		if v.ID == variantID && v.TenantID == tenantID {
			return v.ProductID, v.RegionID, nil
		}
	}
	return "", "", sql.ErrNoRows
}

func (r *catalogFakeRepo) GetProductTypeAttributeVariantOnly(_ context.Context, productTypeID, attributeID, _, _ string) (bool, error) {
	if r.typeAttrLinks == nil || r.typeAttrLinks[productTypeID] == nil {
		return false, sql.ErrNoRows
	}
	st, ok := r.typeAttrLinks[productTypeID][attributeID]
	if !ok {
		return false, sql.ErrNoRows
	}
	return st.variantOnly, nil
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
	if !strings.Contains(body, "tenant_a") || strings.Contains(body, "tenant_b") {
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

func TestAssignProductToCollectionReturnsNotFound(t *testing.T) {
	repo := &catalogFakeRepo{assignErr: ErrAssignEntityNotFound}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/collections/c1/products/p1", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestAssignProductToCollectionReturnsConflictOnDuplicate(t *testing.T) {
	repo := &catalogFakeRepo{assignErr: ErrCollectionProductAlreadyAssigned}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/collections/c1/products/p1", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestProductsListUsesStableCreatedAtCursor(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{
			{ID: "p1", TenantID: "tenant_a", CreatedAt: "2026-04-08T10:00:00Z"},
			{ID: "p2", TenantID: "tenant_a", CreatedAt: "2026-04-08T11:00:00Z"},
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
	var body struct {
		Pagination struct {
			NextCursor string `json:"next_cursor"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body.Pagination.NextCursor != "2026-04-08T11:00:00Z" {
		t.Fatalf("expected stable next cursor, got %q", body.Pagination.NextCursor)
	}
}

func TestProductCreateRejectsDuplicateSlugInTenantRegion(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{
			{ID: "p1", TenantID: "tenant_a", RegionID: "global", Slug: "basic-shirt"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/products/", bytes.NewBufferString(`{"sku":"SKU-2","name":"Shirt","slug":"basic-shirt","currency":"USD","price_cents":100}`))
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	req.Header.Set("Idempotency-Key", "idem-product-duplicate-slug")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}
}

func TestProductsListAppliesLanguageTranslationOverlay(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{
			{ID: "p1", TenantID: "tenant_a", RegionID: "global", Name: "Base Name", Description: "Base Description"},
		},
		productTranslations: map[string]map[string]map[string]string{
			"p1": {
				"fr": {
					"name":            "Nom FR",
					"description":     "Description FR",
					"seo_title":       "SEO FR",
					"seo_description": "Meta FR",
				},
			},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/products/?language_code=fr", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Nom FR") || !strings.Contains(body, "Description FR") || !strings.Contains(body, "SEO FR") || !strings.Contains(body, "Meta FR") {
		t.Fatalf("expected localized values in response, got: %s", body)
	}
}

func TestProductsListFiltersByChannelAndPublishedState(t *testing.T) {
	repo := &catalogFakeRepo{
		items: []Product{
			{ID: "p1", TenantID: "tenant_a", RegionID: "global", Name: "Published Product"},
			{ID: "p2", TenantID: "tenant_a", RegionID: "global", Name: "UNPUBLISHED: Hidden Product"},
			{ID: "p3", TenantID: "tenant_a", RegionID: "global", Name: "Other Channel Product"},
		},
		productChannelFilter: map[string][]string{
			"p1": {"web"},
			"p2": {"web"},
			"p3": {"pos"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/products/?channel_id=web&published_only=true", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Published Product") || strings.Contains(body, "UNPUBLISHED: Hidden Product") || strings.Contains(body, "Other Channel Product") {
		t.Fatalf("expected only published web products, got: %s", body)
	}
}

func TestCategoriesListAppliesLanguageTranslationOverlay(t *testing.T) {
	repo := &catalogFakeRepo{
		categories: []Category{
			{ID: "cat1", TenantID: "tenant_a", RegionID: "global", Name: "Default Category", Slug: "default-category"},
		},
		categoryTranslations: map[string]map[string]map[string]string{
			"cat1": {"fr": {"name": "Categorie FR"}},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/categories/?language_code=fr", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Categorie FR") {
		t.Fatalf("expected localized category name, got: %s", rr.Body.String())
	}
}

func TestCollectionsListAppliesLanguageTranslationOverlay(t *testing.T) {
	repo := &catalogFakeRepo{
		collections: []Collection{
			{ID: "col1", TenantID: "tenant_a", RegionID: "global", Name: "Default Collection", Slug: "default-collection"},
		},
		collectionTranslations: map[string]map[string]map[string]string{
			"col1": {"fr": {"name": "Collection FR"}},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/collections/?language_code=fr", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Collection FR") {
		t.Fatalf("expected localized collection name, got: %s", rr.Body.String())
	}
}

func TestVariantsListFiltersByChannelAndPublishedState(t *testing.T) {
	repo := &catalogFakeRepo{
		variants: []ProductVariant{
			{ID: "v1", TenantID: "tenant_a", RegionID: "global", ProductID: "p1", Name: "Published Variant", SKU: "SKU-1", Currency: "USD", PriceCents: 100},
			{ID: "v2", TenantID: "tenant_a", RegionID: "global", ProductID: "p1", Name: "UNPUBLISHED: Hidden Variant", SKU: "SKU-2", Currency: "USD", PriceCents: 110},
			{ID: "v3", TenantID: "tenant_a", RegionID: "global", ProductID: "p1", Name: "Other Channel Variant", SKU: "SKU-3", Currency: "USD", PriceCents: 120},
		},
		variantChannelFilter: map[string][]string{
			"v1": {"web"},
			"v2": {"web"},
			"v3": {"pos"},
		},
	}
	h := NewHandler(NewService(repo, events.NewBus()))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/products/p1/variants/?channel_id=web&published_only=true", nil)
	req.Header.Set("X-Tenant-ID", "tenant_a")
	req.Header.Set("X-Region-ID", "global")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Published Variant") || strings.Contains(body, "UNPUBLISHED: Hidden Variant") || strings.Contains(body, "Other Channel Variant") {
		t.Fatalf("expected only published web variants, got: %s", body)
	}
}
