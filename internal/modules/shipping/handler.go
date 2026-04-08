package shipping

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
	r.Route("/shipping/zones", func(r chi.Router) {
		r.Get("/", h.listZones)
		r.Post("/", h.saveZone)
	})
	r.Route("/shipping/methods", func(r chi.Router) {
		r.Get("/", h.listMethods)
		r.Post("/", h.saveMethod)
	})
	r.Post("/shipping/resolve", h.resolve)
}

func (h *Handler) listZones(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListZones(r.Context(), middleware.TenantIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, items)
}

func (h *Handler) saveZone(w http.ResponseWriter, r *http.Request) {
	var z ShippingZone
	if err := json.NewDecoder(r.Body).Decode(&z); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	z.TenantID = middleware.TenantIDFromContext(r.Context())
	z.RegionID = middleware.RegionIDFromContext(r.Context())
	saved, err := h.svc.SaveZone(r.Context(), z)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listMethods(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListMethods(r.Context(), middleware.TenantIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, items)
}

func (h *Handler) saveMethod(w http.ResponseWriter, r *http.Request) {
	var m ShippingMethod
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	m.TenantID = middleware.TenantIDFromContext(r.Context())
	m.RegionID = middleware.RegionIDFromContext(r.Context())
	if m.ID == "" {
		m.ID = utils.NewID("shm")
	}
	saved, err := h.svc.SaveMethod(r.Context(), m)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	var in ResolveInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	items, err := h.svc.ResolveEligible(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		in,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}
