package shop

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
	r.Route("/shop", func(r chi.Router) {
		r.Get("/settings", h.getSettings)
		r.Patch("/settings", h.patchSettings)
	})
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, s)
}

func (h *Handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	var patch PatchInput
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	s, err := h.svc.Patch(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()), patch, key)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, s)
}
