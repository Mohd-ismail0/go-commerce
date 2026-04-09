package webhooks

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
	r.Route("/apps/webhook-subscriptions", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Patch("/", h.patch)
		r.Post("/{subscriptionID}/activate", h.activate)
		r.Post("/{subscriptionID}/deactivate", h.deactivate)
	})
	r.Route("/webhooks", func(r chi.Router) {
		r.Get("/deliveries", h.listDeliveries)
		r.Post("/outbox/{outboxID}/retry", h.retryOutbox)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	onlyActive := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active")), "true")
	items, err := h.svc.List(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		onlyActive,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req Subscription
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	if req.ID == "" {
		req.ID = utils.NewID("whs")
	}
	req.TenantID = middleware.TenantIDFromContext(r.Context())
	req.RegionID = middleware.RegionIDFromContext(r.Context())
	saved, err := h.svc.Save(r.Context(), req, false)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	var req Subscription
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "id is required for patch"})
		return
	}
	req.TenantID = middleware.TenantIDFromContext(r.Context())
	req.RegionID = middleware.RegionIDFromContext(r.Context())
	saved, err := h.svc.Save(r.Context(), req, true)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

func (h *Handler) deactivate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	saved, err := h.svc.SetActive(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "subscriptionID"),
		active,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) listDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	eventName := strings.TrimSpace(r.URL.Query().Get("event_name"))
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	items, err := h.svc.ListDeliveries(r.Context(), tenantID, regionID, status, eventName, limit)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) retryOutbox(w http.ResponseWriter, r *http.Request) {
	err := h.svc.RetryDeadOutbox(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "outboxID"),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"retried": true})
}
