package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	limit := int32(20)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = int32(parsed)
		}
	}
	var cursor *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid cursor"})
			return
		}
		cursor = &parsed
	}
	items, err := h.svc.List(r.Context(), tenantID, regionID, sku, cursor, limit)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	nextCursor := ""
	if len(items) > 0 {
		nextCursor = time.Now().UTC().Format(time.RFC3339Nano)
	}
	utils.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"next_cursor": nextCursor,
		},
	})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	if p.ID == "" {
		p.ID = utils.NewID("prd")
	}
	saved, err := h.svc.Save(r.Context(), p, idempotencyKey)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
