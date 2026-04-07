package catalog

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
	r.Route("/products", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.upsert)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	sku := strings.TrimSpace(r.URL.Query().Get("sku"))
	items, err := h.svc.List(r.Context(), tenantID, regionID, sku)
	if err != nil {
		utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list products"})
		return
	}
	utils.JSON(w, http.StatusOK, items)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	if p.ID == "" {
		p.ID = utils.NewID("prd")
	}
	saved, err := h.svc.Save(r.Context(), p)
	if err != nil {
		utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save product"})
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
