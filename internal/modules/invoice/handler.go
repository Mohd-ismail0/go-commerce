package invoice

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
	r.Route("/invoices", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{invoice_id}", h.get)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.URL.Query().Get("order_id"))
	items, err := h.svc.ListByOrder(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()), orderID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	inv, err := h.svc.Get(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()), chi.URLParam(r, "invoice_id"))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, inv)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             string          `json:"id"`
		OrderID        string          `json:"order_id"`
		InvoiceNumber  string          `json:"invoice_number"`
		Status         string          `json:"status"`
		Metadata       json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.Create(r.Context(), middleware.TenantIDFromContext(r.Context()), middleware.RegionIDFromContext(r.Context()), CreateInput{
		ID: req.ID, OrderID: req.OrderID, InvoiceNumber: req.InvoiceNumber, Status: req.Status, Metadata: req.Metadata,
	}, key)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
