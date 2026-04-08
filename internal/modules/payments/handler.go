package payments

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"rewrite/internal/shared/middleware"
	"rewrite/internal/shared/utils"
)

type Handler struct {
	svc           *Service
	webhookSecret string
	appEnv        string
}

func NewHandler(svc *Service, webhookSecret, appEnv string) *Handler {
	return &Handler{svc: svc, webhookSecret: strings.TrimSpace(webhookSecret), appEnv: strings.ToLower(strings.TrimSpace(appEnv))}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/payments", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Get("/{id}", h.get)
		r.Get("/{id}/transactions", h.transactions)
		r.Post("/{id}/capture", h.capture)
		r.Post("/{id}/refund", h.refund)
		r.Post("/{id}/void", h.void)
	})
	r.Post("/webhooks/payments/{tenant_id}/{provider}", h.webhook)
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
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	var item Payment
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	item.TenantID = middleware.TenantIDFromContext(r.Context())
	item.RegionID = middleware.RegionIDFromContext(r.Context())
	if item.ID == "" {
		item.ID = utils.NewID("pay")
	}
	saved, err := h.svc.Save(r.Context(), item, idem)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), middleware.TenantIDFromContext(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, p)
}

func (h *Handler) transactions(w http.ResponseWriter, r *http.Request) {
	txs, err := h.svc.ListTransactions(r.Context(), middleware.TenantIDFromContext(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, txs)
}

func (h *Handler) capture(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AmountCents *int64 `json:"amount_cents"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := h.svc.Capture(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		chi.URLParam(r, "id"),
		AmountActionInput{AmountCents: body.AmountCents},
		strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AmountCents *int64 `json:"amount_cents"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := h.svc.Refund(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		chi.URLParam(r, "id"),
		AmountActionInput{AmountCents: body.AmountCents},
		strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}

func (h *Handler) void(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Void(
		r.Context(),
		middleware.TenantIDFromContext(r.Context()),
		chi.URLParam(r, "id"),
		strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	sig := strings.TrimSpace(r.Header.Get("X-Webhook-Signature"))
	if h.webhookSecret != "" && !VerifyWebhookSignature(h.webhookSecret, body, sig) {
		utils.JSON(w, http.StatusUnauthorized, map[string]any{"code": "unauthorized", "message": "invalid webhook signature"})
		return
	}
	if h.webhookSecret == "" && (h.appEnv == "production" || h.appEnv == "prod") {
		utils.JSON(w, http.StatusServiceUnavailable, map[string]any{"code": "auth_unavailable", "message": "webhook signing secret is not configured"})
		return
	}

	var in WebhookInput
	if err := json.Unmarshal(body, &in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid json"})
		return
	}

	tenantID := strings.TrimSpace(chi.URLParam(r, "tenant_id"))
	if tenantID == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "tenant_id path segment is required"})
		return
	}
	regionID := strings.TrimSpace(r.Header.Get("X-Region-ID"))
	if regionID == "" {
		regionID = strings.TrimSpace(r.URL.Query().Get("region_id"))
	}
	if regionID == "" {
		regionID = middleware.RegionIDFromContext(r.Context())
	}

	ctx := middleware.WithTenantID(middleware.WithRegionID(r.Context(), regionID), tenantID)
	res, err := h.svc.ProcessWebhook(ctx, tenantID, regionID, in)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, res)
}
