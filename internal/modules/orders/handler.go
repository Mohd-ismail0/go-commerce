package orders

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
	r.Route("/orders", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Patch("/", h.updateStatus)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
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
	items, err := h.svc.List(r.Context(), tenantID, regionID, cursor, limit)
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

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	o.TenantID = middleware.TenantIDFromContext(r.Context())
	o.RegionID = middleware.RegionIDFromContext(r.Context())
	if o.ID == "" {
		o.ID = utils.NewID("ord")
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.Create(r.Context(), o, idempotencyKey)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID                string `json:"id"`
		Status            string `json:"status"`
		ExpectedUpdatedAt string `json:"expected_updated_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	expected, err := time.Parse(time.RFC3339Nano, req.ExpectedUpdatedAt)
	if err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid expected_updated_at"})
		return
	}
	updated, err := h.svc.UpdateStatus(r.Context(), tenantID, StatusUpdateInput{
		ID:                req.ID,
		Status:            req.Status,
		ExpectedUpdatedAt: expected,
	})
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, updated)
}
