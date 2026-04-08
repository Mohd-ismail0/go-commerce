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
		r.Get("/rules", h.listRules)
		r.Post("/rules", h.saveRule)
		r.Get("/vouchers", h.listVouchers)
		r.Post("/vouchers", h.saveVoucher)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var p Promotion
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.Save(r.Context(), p))
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.ListRules(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) saveRule(w http.ResponseWriter, r *http.Request) {
	var p PromotionRule
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.SaveRule(r.Context(), p))
}

func (h *Handler) listVouchers(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.ListVouchers(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) saveVoucher(w http.ResponseWriter, r *http.Request) {
	var v Voucher
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	v.TenantID = middleware.TenantIDFromContext(r.Context())
	v.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.SaveVoucher(r.Context(), v))
}
