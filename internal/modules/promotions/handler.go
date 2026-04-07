package promotions

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
	r.Route("/promotions", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var p Promotion
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.Save(r.Context(), p))
}
