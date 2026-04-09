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
		r.Post("/", h.create)
		r.Patch("/", h.patch)
		r.Post("/{appID}/activate", h.activate)
		r.Post("/{appID}/deactivate", h.deactivate)
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

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
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
	saved, err := h.svc.Save(r.Context(), item, false)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(req.ID) == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "id is required for patch"})
		return
	}
	item := App{
		ID:        strings.TrimSpace(req.ID),
		TenantID:  middleware.TenantIDFromContext(r.Context()),
		RegionID:  middleware.RegionIDFromContext(r.Context()),
		Name:      req.Name,
		AuthToken: req.AuthToken,
	}
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}
	saved, err := h.svc.Save(r.Context(), item, true)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

func (h *Handler) deactivate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	saved, err := h.svc.SetActive(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "appID"),
		active,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}
