package apps

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
	r.Route("/apps", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Patch("/", h.save)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active")), "true")
	items, err := h.svc.List(r.Context(), tenantID, regionID, activeOnly)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsActive  *bool  `json:"is_active"`
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	item := App{
		ID:        strings.TrimSpace(req.ID),
		TenantID:  middleware.TenantIDFromContext(r.Context()),
		RegionID:  middleware.RegionIDFromContext(r.Context()),
		Name:      req.Name,
		AuthToken: req.AuthToken,
	}
	if item.ID == "" {
		item.ID = utils.NewID("app")
	}
	item.IsActive = true
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}
	saved, err := h.svc.Save(r.Context(), item)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
