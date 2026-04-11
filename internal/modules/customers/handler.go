package customers

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
	r.Route("/customers", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Route("/{customer_id}/addresses", func(r chi.Router) {
			r.Get("/", h.listAddresses)
			r.Post("/", h.createAddress)
			r.Put("/{address_id}", h.putAddress)
			r.Delete("/{address_id}", h.deleteAddress)
		})
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, items)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var c Customer
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	c.TenantID = middleware.TenantIDFromContext(r.Context())
	c.RegionID = middleware.RegionIDFromContext(r.Context())
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.Save(r.Context(), c, idempotencyKey)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")
	items, err := h.svc.ListAddresses(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		customerID,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createAddress(w http.ResponseWriter, r *http.Request) {
	var body Address
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	customerID := chi.URLParam(r, "customer_id")
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.CreateAddress(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		customerID,
		idempotencyKey,
		body,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) putAddress(w http.ResponseWriter, r *http.Request) {
	var body Address
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	customerID := chi.URLParam(r, "customer_id")
	addressID := chi.URLParam(r, "address_id")
	saved, err := h.svc.ReplaceAddress(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		customerID,
		addressID,
		body,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")
	addressID := chi.URLParam(r, "address_id")
	if err := h.svc.DeleteAddress(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		customerID,
		addressID,
	); err != nil {
		utils.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
