package webhooks

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
	r.Route("/apps/webhook-subscriptions", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Patch("/", h.save)
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

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
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
	saved, err := h.svc.Save(r.Context(), req)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
