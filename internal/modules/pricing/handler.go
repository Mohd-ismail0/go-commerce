package pricing

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
	r.Route("/pricing", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Get("/tax-classes", h.listTaxClasses)
		r.Post("/tax-classes", h.saveTaxClass)
		r.Get("/tax-rates", h.listTaxRates)
		r.Post("/tax-rates", h.saveTaxRate)
		r.Post("/calculate", h.calculate)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.List(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var p PriceBookEntry
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.Save(r.Context(), p))
}

func (h *Handler) listTaxClasses(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.ListTaxClasses(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) saveTaxClass(w http.ResponseWriter, r *http.Request) {
	var p TaxClass
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.SaveTaxClass(r.Context(), p))
}

func (h *Handler) listTaxRates(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, h.svc.ListTaxRates(r.Context(), middleware.TenantIDFromContext(r.Context())))
}

func (h *Handler) saveTaxRate(w http.ResponseWriter, r *http.Request) {
	var p TaxRate
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusCreated, h.svc.SaveTaxRate(r.Context(), p))
}

func (h *Handler) calculate(w http.ResponseWriter, r *http.Request) {
	var input CalculationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	input.TenantID = middleware.TenantIDFromContext(r.Context())
	input.RegionID = middleware.RegionIDFromContext(r.Context())
	utils.JSON(w, http.StatusOK, h.svc.Calculate(r.Context(), input))
}
