package orders

import (
	"encoding/json"
	"errors"
	"net/http"

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
	items, err := h.svc.List(r.Context(), tenantID, regionID)
	if err != nil {
		utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list orders"})
		return
	}
	utils.JSON(w, http.StatusOK, items)
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
	saved, err := h.svc.Create(r.Context(), o)
	if err != nil {
		utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create order"})
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	updated, err := h.svc.UpdateStatus(r.Context(), tenantID, req.ID, req.Status)
	if err != nil {
		if errors.Is(err, ErrInvalidStatusTransition) {
			utils.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update order"})
		return
	}
	utils.JSON(w, http.StatusOK, updated)
}
