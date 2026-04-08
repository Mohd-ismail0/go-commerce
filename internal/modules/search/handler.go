package search

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	r.Get("/search", h.query)
	r.Post("/search/index", h.save)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var item Document
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	item.TenantID = middleware.TenantIDFromContext(r.Context())
	item.RegionID = middleware.RegionIDFromContext(r.Context())
	if item.ID == "" {
		item.ID = utils.NewID("sdc")
	}
	utils.JSON(w, http.StatusCreated, h.svc.Save(r.Context(), item))
}

func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	entityType := strings.TrimSpace(r.URL.Query().Get("entity_type"))
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	items := h.svc.Query(r.Context(), tenantID, regionID, entityType, q, limit)
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}
