package fulfillments

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
	r.Route("/fulfillments", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             string `json:"id"`
		OrderID        string `json:"order_id"`
		Status         string `json:"status"`
		TrackingNumber string `json:"tracking_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	id := req.ID
	if id == "" {
		id = utils.NewID("ful")
	}
	saved, err := h.svc.Create(r.Context(), Fulfillment{
		ID:             id,
		TenantID:       middleware.TenantIDFromContext(r.Context()),
		RegionID:       middleware.RegionIDFromContext(r.Context()),
		OrderID:        req.OrderID,
		Status:         req.Status,
		TrackingNumber: req.TrackingNumber,
	})
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.URL.Query().Get("order_id"))
	items, err := h.svc.List(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		orderID,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}
