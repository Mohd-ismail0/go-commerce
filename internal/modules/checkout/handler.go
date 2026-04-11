package checkout

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
	r.Route("/checkouts", func(r chi.Router) {
		r.Post("/sessions", h.createSession)
		r.Get("/sessions/{checkout_id}", h.getSession)
		r.Patch("/sessions/{checkout_id}", h.updateSessionContext)
		r.Get("/sessions/{checkout_id}/lines", h.listLines)
		r.Put("/sessions/{checkout_id}/lines", h.upsertLine)
		r.Post("/sessions/{checkout_id}/recalculate", h.recalculate)
		r.Post("/sessions/{checkout_id}/apply-customer-addresses", h.applyCustomerAddresses)
		r.Post("/sessions/{checkout_id}/complete", h.complete)
	})
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID                        string `json:"id"`
		CustomerID                string `json:"customer_id"`
		ChannelID                 string `json:"channel_id"`
		ShippingMethodID          string `json:"shipping_method_id"`
		ShippingAddressCountry    string `json:"shipping_address_country"`
		ShippingAddressPostalCode string `json:"shipping_address_postal_code"`
		BillingAddressCountry     string `json:"billing_address_country"`
		BillingAddressPostalCode  string `json:"billing_address_postal_code"`
		Currency                  string `json:"currency"`
		VoucherCode               string `json:"voucher_code"`
		PromotionID               string `json:"promotion_id"`
		TaxClassID                string `json:"tax_class_id"`
		CountryCode               string `json:"country_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	id := req.ID
	if id == "" {
		id = utils.NewID("chk")
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.CreateSession(r.Context(), Session{
		ID:                        id,
		TenantID:                  middleware.TenantIDFromContext(r.Context()),
		RegionID:                  middleware.RegionIDFromContext(r.Context()),
		CustomerID:                req.CustomerID,
		ChannelID:                 req.ChannelID,
		ShippingMethodID:          req.ShippingMethodID,
		ShippingAddressCountry:    req.ShippingAddressCountry,
		ShippingAddressPostalCode: req.ShippingAddressPostalCode,
		BillingAddressCountry:     req.BillingAddressCountry,
		BillingAddressPostalCode:  req.BillingAddressPostalCode,
		Currency:                  req.Currency,
		VoucherCode:               req.VoucherCode,
		PromotionID:               req.PromotionID,
		TaxClassID:                req.TaxClassID,
		CountryCode:               req.CountryCode,
	}, idempotencyKey)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	sess, err := h.svc.GetSession(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) listLines(w http.ResponseWriter, r *http.Request) {
	lines, err := h.svc.ListLines(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": lines})
}

func (h *Handler) upsertLine(w http.ResponseWriter, r *http.Request) {
	var req Line
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
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
		VoucherCode               string `json:"voucher_code"`
		PromotionID               string `json:"promotion_id"`
		TaxClassID                string `json:"tax_class_id"`
		CountryCode               string `json:"country_code"`
		ChannelID                 string `json:"channel_id"`
		ShippingMethodID          string `json:"shipping_method_id"`
		ShippingAddressCountry    string `json:"shipping_address_country"`
		ShippingAddressPostalCode string `json:"shipping_address_postal_code"`
		BillingAddressCountry     string `json:"billing_address_country"`
		BillingAddressPostalCode  string `json:"billing_address_postal_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	updated, err := h.svc.UpdateSessionContext(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
		Session{
			VoucherCode:               req.VoucherCode,
			PromotionID:               req.PromotionID,
			TaxClassID:                req.TaxClassID,
			CountryCode:               req.CountryCode,
			ChannelID:                 req.ChannelID,
			ShippingMethodID:          req.ShippingMethodID,
			ShippingAddressCountry:    req.ShippingAddressCountry,
			ShippingAddressPostalCode: req.ShippingAddressPostalCode,
			BillingAddressCountry:     req.BillingAddressCountry,
			BillingAddressPostalCode:  req.BillingAddressPostalCode,
		},
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, updated)
}

func (h *Handler) applyCustomerAddresses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ShippingAddressID string `json:"shipping_address_id"`
		BillingAddressID  string `json:"billing_address_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	updated, err := h.svc.ApplyCustomerAddresses(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
		req.ShippingAddressID,
		req.BillingAddressID,
		idempotencyKey,
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
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	saved, err := h.svc.Complete(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		middleware.RegionIDFromContext(r.Context()),
		chi.URLParam(r, "checkout_id"),
		idempotencyKey,
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}
