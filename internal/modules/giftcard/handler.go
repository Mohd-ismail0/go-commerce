package giftcard

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
	r.Route("/gift-cards", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()))
	if err != nil {
		utils.JSON(w, http.StatusInternalServerError, map[string]any{"code": "internal", "message": "failed to list gift cards"})
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID               string `json:"id"`
		Code             string `json:"code"`
		BalanceCents     int64  `json:"balance_cents"`
		Currency         string `json:"currency"`
		IsActive         *bool  `json:"is_active"`
		ExpiresAtRFC3339 string `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	saved, err := h.svc.Create(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()), CreateInput{
		ID:               req.ID,
		Code:             req.Code,
		BalanceCents:     req.BalanceCents,
		Currency:         req.Currency,
		IsActive:         active,
		ExpiresAtRFC3339: req.ExpiresAtRFC3339,
	}, key)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
