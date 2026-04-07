package orders

import (
	"encoding/json"
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
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), tenantID))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	o.TenantID = middleware.TenantIDFromContext(r.Context())
	o.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.Create(r.Context(), o))
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	o.TenantID = middleware.TenantIDFromContext(r.Context())
	o.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusOK, h.svc.UpdateStatus(r.Context(), o))
}
