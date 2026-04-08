package checkout

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
	r.Route("/checkouts", func(r chi.Router) {
		r.Post("/sessions", h.createSession)
		r.Patch("/sessions/{checkout_id}", h.updateSessionContext)
		r.Put("/sessions/{checkout_id}/lines", h.upsertLine)
		r.Post("/sessions/{checkout_id}/recalculate", h.recalculate)
		r.Post("/sessions/{checkout_id}/complete", h.complete)
	})
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         string `json:"id"`
		CustomerID string `json:"customer_id"`
		Currency   string `json:"currency"`
		VoucherCode string `json:"voucher_code"`
		PromotionID string `json:"promotion_id"`
		TaxClassID  string `json:"tax_class_id"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	id := req.ID
	if id == "" {
		id = utils.NewID("chk")
	}
	saved, err := h.svc.CreateSession(r.Context(), Session{
		ID:         id,
		TenantID:   middleware.TenantIDFromContext(r.Context()),
		RegionID:   middleware.RegionIDFromContext(r.Context()),
		CustomerID: req.CustomerID,
		Currency:   req.Currency,
		VoucherCode: req.VoucherCode,
		PromotionID: req.PromotionID,
		TaxClassID:  req.TaxClassID,
		CountryCode: req.CountryCode,
	})
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) upsertLine(w http.ResponseWriter, r *http.Request) {
	var req Line
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	req.CheckoutID = chi.URLParam(r, "checkout_id")
	saved, err := h.svc.UpsertLine(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		req,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) updateSessionContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VoucherCode string `json:"voucher_code"`
		PromotionID string `json:"promotion_id"`
		TaxClassID  string `json:"tax_class_id"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	updated, err := h.svc.UpdateSessionContext(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
		Session{
			VoucherCode: req.VoucherCode,
			PromotionID: req.PromotionID,
			TaxClassID:  req.TaxClassID,
			CountryCode: req.CountryCode,
		},
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, updated)
}

func (h *Handler) recalculate(w http.ResponseWriter, r *http.Request) {
	saved, err := h.svc.Recalculate(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, saved)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	saved, err := h.svc.Complete(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
