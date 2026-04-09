package channels

import (
	"bytes"
	"context"
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
