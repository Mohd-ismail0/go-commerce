package channels

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/middleware"
	"rewrite/internal/shared/utils"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/channels", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Patch("/", h.save)
		r.Route("/{channelID}/product-listings", func(r chi.Router) {
			r.Get("/", h.listProductListings)
			r.Post("/", h.saveProductListing)
			r.Patch("/", h.patchProductListing)
		})
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	items, err := h.svc.List(r.Context(), tenantID, regionID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID              string `json:"id"`
		Slug            string `json:"slug"`
		Name            string `json:"name"`
		DefaultCurrency string `json:"default_currency"`
		DefaultCountry  string `json:"default_country"`
		IsActive        *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	ch := Channel{
		ID:              strings.TrimSpace(req.ID),
		TenantID:        middleware.TenantIDFromContext(r.Context()),
		RegionID:        middleware.RegionIDFromContext(r.Context()),
		Slug:            req.Slug,
		Name:            req.Name,
		DefaultCurrency: req.DefaultCurrency,
		DefaultCountry:  req.DefaultCountry,
		IsActive:        true,
	}
	if req.IsActive != nil {
		ch.IsActive = *req.IsActive
	}
	saved, err := h.svc.Save(r.Context(), ch)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listProductListings(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	channelID := strings.TrimSpace(chi.URLParam(r, "channelID"))
	items, err := h.svc.ListProductListings(r.Context(), tenantID, regionID, channelID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) saveProductListing(w http.ResponseWriter, r *http.Request) {
	h.writeProductListing(w, r, false)
}

func (h *Handler) patchProductListing(w http.ResponseWriter, r *http.Request) {
	h.writeProductListing(w, r, true)
}

func (h *Handler) writeProductListing(w http.ResponseWriter, r *http.Request, patch bool) {
	var req struct {
		ID                string  `json:"id"`
		ProductID         string  `json:"product_id"`
		PublishedAt       *string `json:"published_at"`
		IsPublished       *bool   `json:"is_published"`
		VisibleInListings *bool   `json:"visible_in_listings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	channelID := strings.TrimSpace(chi.URLParam(r, "channelID"))
	saved, err := h.svc.UpsertProductListing(r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		channelID,
		ProductListingInput{
			ID:                req.ID,
			ProductID:         req.ProductID,
			PublishedAt:       req.PublishedAt,
			IsPublished:       req.IsPublished,
			VisibleInListings: req.VisibleInListings,
		},
		patch,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	status := http.StatusCreated
	if patch {
		status = http.StatusOK
	}
	utils.JSON(w, status, saved)
}
