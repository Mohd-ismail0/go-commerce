package channels

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/middleware"
)

type stubChannelRepo struct {
	listRet  []Channel
	listErr  error
	taken    bool
	takenErr error
	saveRet  Channel
	saveErr  error

	channelExists       bool
	productExists       bool
	listingByChannelRet []ProductChannelListing
	getListing          ProductChannelListing
	getListingOK        bool
	saveListingRet      ProductChannelListing
}

func (s *stubChannelRepo) SlugTaken(_ context.Context, _, _, _, _ string) (bool, error) {
	return s.taken, s.takenErr
}

func (s *stubChannelRepo) Save(_ context.Context, ch Channel) (Channel, error) {
	if s.saveErr != nil {
		return Channel{}, s.saveErr
	}
	if s.saveRet.ID != "" {
		return s.saveRet, nil
	}
	return ch, nil
}

func (s *stubChannelRepo) List(_ context.Context, tenantID, regionID string) ([]Channel, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]Channel, 0, len(s.listRet))
	for _, c := range s.listRet {
		if c.TenantID == tenantID && c.RegionID == regionID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *stubChannelRepo) ChannelExists(_ context.Context, _, _, _ string) (bool, error) {
	return s.channelExists, nil
}

func (s *stubChannelRepo) ProductExists(_ context.Context, _, _, _ string) (bool, error) {
	return s.productExists, nil
}

func (s *stubChannelRepo) GetProductListingByKeys(_ context.Context, _, _, _, _ string) (ProductChannelListing, bool, error) {
	return s.getListing, s.getListingOK, nil
}

func (s *stubChannelRepo) ListProductListingsByChannel(_ context.Context, _, _, _ string) ([]ProductChannelListing, error) {
	return s.listingByChannelRet, nil
}

func (s *stubChannelRepo) SaveProductListing(_ context.Context, row ProductChannelListing, _ sql.NullTime) (ProductChannelListing, error) {
	if s.saveListingRet.ID != "" {
		return s.saveListingRet, nil
	}
	return row, nil
}

func TestChannelsListTenantRegionScoped(t *testing.T) {
	repo := &stubChannelRepo{
		listRet: []Channel{
			{ID: "c1", TenantID: "t1", RegionID: "r1", Slug: "default", Name: "Default"},
			{ID: "c2", TenantID: "t2", RegionID: "r1", Slug: "other", Name: "Other"},
		},
	}
	h := NewHandler(NewService(repo))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/channels/", nil)
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("X-Region-ID", "r1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "c1") || strings.Contains(body, "c2") {
		t.Fatalf("expected only tenant t1 channels in response: %s", body)
	}
}

func TestChannelsCreateReturnsConflictOnDuplicateSlug(t *testing.T) {
	repo := &stubChannelRepo{taken: true}
	h := NewHandler(NewService(repo))
	r := chi.NewRouter()
	r.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/channels/", bytes.NewBufferString(
		`{"slug":"web","name":"Web Store","default_currency":"USD","default_country":"US"}`,
	))
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("X-Region-ID", "r1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProductListingsListRequiresExistingChannel(t *testing.T) {
	repo := &stubChannelRepo{channelExists: false}
	h := NewHandler(NewService(repo))
	rt := chi.NewRouter()
	rt.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(rt)

	req := httptest.NewRequest(http.MethodGet, "/channels/c1/product-listings", nil)
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("X-Region-ID", "r1")
	rr := httptest.NewRecorder()
	rt.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProductListingsPatchNotFoundWhenMissing(t *testing.T) {
	repo := &stubChannelRepo{
		channelExists: true,
		productExists: true,
		getListingOK:  false,
	}
	h := NewHandler(NewService(repo))
	rt := chi.NewRouter()
	rt.Use(middleware.TenantRegion("public", "global"))
	h.RegisterRoutes(rt)

	req := httptest.NewRequest(http.MethodPatch, "/channels/c1/product-listings", bytes.NewBufferString(
		`{"product_id":"p1","is_published":true}`,
	))
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("X-Region-ID", "r1")
	rr := httptest.NewRecorder()
	rt.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}
