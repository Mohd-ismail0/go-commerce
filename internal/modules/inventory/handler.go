package inventory

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
	r.Route("/inventory", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Get("/warehouses", h.listWarehouses)
		r.Post("/warehouses", h.saveWarehouse)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var i StockItem
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	i.TenantID = middleware.TenantIDFromContext(r.Context())
	i.RegionID = middleware.RegionIDFromContext(r.Context())
	saved := h.svc.Save(r.Context(), i)
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listWarehouses(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListWarehouses(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) saveWarehouse(w http.ResponseWriter, r *http.Request) {
	var wbody Warehouse
	if err := json.NewDecoder(r.Body).Decode(&wbody); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	wbody.TenantID = middleware.TenantIDFromContext(r.Context())
	wbody.RegionID = middleware.RegionIDFromContext(r.Context())
	saved, err := h.svc.SaveWarehouse(r.Context(), wbody)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
