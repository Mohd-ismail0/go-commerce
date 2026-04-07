package payments

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
	r.Route("/payments", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var item Payment
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	item.TenantID = middleware.TenantIDFromContext(r.Context())
	item.RegionID = middleware.RegionIDFromContext(r.Context())
	if item.ID == "" {
		item.ID = utils.NewID("pay")
	}
	utils.JSON(w, http.StatusCreated, h.svc.Save(r.Context(), item))
}
