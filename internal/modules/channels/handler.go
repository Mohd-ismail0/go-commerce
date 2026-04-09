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
