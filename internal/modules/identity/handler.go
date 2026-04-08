package identity

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
	r.Route("/identity/auth", func(r chi.Router) {
		r.Post("/login", h.login)
		r.Post("/refresh", h.refresh)
		r.Post("/logout", h.logout)
	})
	r.Route("/identity/users", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, items)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var item User
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	item.TenantID = middleware.TenantIDFromContext(r.Context())
	item.RegionID = middleware.RegionIDFromContext(r.Context())
	if item.ID == "" {
		item.ID = utils.NewID("usr")
	}
	saved, err := h.svc.Save(r.Context(), item)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	res, err := h.svc.Login(r.Context(), middleware.TenantIDFromContext(r.Context()), in)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var in RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	res, err := h.svc.Refresh(r.Context(), middleware.TenantIDFromContext(r.Context()), in)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var in RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	if err := h.svc.Logout(r.Context(), middleware.TenantIDFromContext(r.Context()), in); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
