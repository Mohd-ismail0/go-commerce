package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	r.Route("/product-types", func(r chi.Router) {
		r.Get("/", h.listProductTypes)
		r.Post("/", h.createProductType)
		r.Get("/{productTypeID}", h.getProductTypeDetail)
		r.Post("/{productTypeID}/attributes", h.linkAttributeToProductType)
		r.Delete("/{productTypeID}/attributes/{attributeID}", h.unlinkAttributeFromProductType)
	})
	r.Route("/attributes", func(r chi.Router) {
		r.Get("/", h.listCatalogAttributes)
		r.Post("/", h.createCatalogAttribute)
	})
	r.Route("/products", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.upsert)
		r.Route("/{productID}/attribute-values", func(r chi.Router) {
			r.Get("/", h.listProductAttributeValues)
			r.Put("/", h.putProductAttributeValues)
		})
		r.Route("/{productID}/variants", func(r chi.Router) {
			r.Get("/", h.listVariants)
			r.Post("/", h.upsertVariant)
			r.Patch("/", h.upsertVariant)
			r.Route("/{variantID}/attribute-values", func(r chi.Router) {
				r.Get("/", h.listVariantAttributeValues)
				r.Put("/", h.putVariantAttributeValues)
			})
		})
		r.Route("/{productID}/media", func(r chi.Router) {
			r.Get("/", h.listProductMedia)
			r.Post("/", h.createProductMedia)
		})
	})
	r.Route("/categories", func(r chi.Router) {
		r.Get("/", h.listCategories)
		r.Post("/", h.createCategory)
	})
	r.Route("/collections", func(r chi.Router) {
		r.Get("/", h.listCollections)
		r.Post("/", h.createCollection)
		r.Post("/{collectionID}/products/{productID}", h.assignProductToCollection)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	sku := strings.TrimSpace(r.URL.Query().Get("sku"))
	languageCode := strings.TrimSpace(r.URL.Query().Get("language_code"))
	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	publishedOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("published_only")), "true")
	expandAttributeValues := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("expand")), "attribute_values")
	limit := int32(20)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = int32(parsed)
		}
	}
	var cursor *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid cursor"})
			return
		}
		cursor = &parsed
	}
	items, err := h.svc.List(r.Context(), tenantID, regionID, sku, languageCode, channelID, publishedOnly, cursor, limit, expandAttributeValues)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	nextCursor := ""
	if len(items) > 0 {
		last := items[len(items)-1]
		if strings.TrimSpace(last.CreatedAt) != "" {
			nextCursor = last.CreatedAt
		}
	}
	utils.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"next_cursor": nextCursor,
		},
	})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	p.TenantID = middleware.TenantIDFromContext(r.Context())
	p.RegionID = middleware.RegionIDFromContext(r.Context())
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "Idempotency-Key header is required"})
		return
	}
	if p.ID == "" {
		p.ID = utils.NewID("prd")
	}
	saved, err := h.svc.Save(r.Context(), p, idempotencyKey)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) upsertVariant(w http.ResponseWriter, r *http.Request) {
	var v ProductVariant
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	v.TenantID = middleware.TenantIDFromContext(r.Context())
	v.RegionID = middleware.RegionIDFromContext(r.Context())
	v.ProductID = chi.URLParam(r, "productID")
	if v.ID == "" {
		v.ID = utils.NewID("var")
	}
	saved, err := h.svc.SaveVariant(r.Context(), v)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listVariants(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	publishedOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("published_only")), "true")
	items, err := h.svc.ListVariants(r.Context(), tenantID, regionID, productID, channelID, publishedOnly)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var category Category
	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	category.TenantID = middleware.TenantIDFromContext(r.Context())
	category.RegionID = middleware.RegionIDFromContext(r.Context())
	if category.ID == "" {
		category.ID = utils.NewID("cat")
	}
	saved, err := h.svc.SaveCategory(r.Context(), category)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	languageCode := strings.TrimSpace(r.URL.Query().Get("language_code"))
	items, err := h.svc.ListCategories(r.Context(), tenantID, regionID, languageCode)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createCollection(w http.ResponseWriter, r *http.Request) {
	var collection Collection
	if err := json.NewDecoder(r.Body).Decode(&collection); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	collection.TenantID = middleware.TenantIDFromContext(r.Context())
	collection.RegionID = middleware.RegionIDFromContext(r.Context())
	if collection.ID == "" {
		collection.ID = utils.NewID("col")
	}
	saved, err := h.svc.SaveCollection(r.Context(), collection)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listCollections(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	languageCode := strings.TrimSpace(r.URL.Query().Get("language_code"))
	items, err := h.svc.ListCollections(r.Context(), tenantID, regionID, languageCode)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) assignProductToCollection(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	collectionID := chi.URLParam(r, "collectionID")
	productID := chi.URLParam(r, "productID")
	if err := h.svc.AssignProductToCollection(r.Context(), tenantID, regionID, collectionID, productID); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, map[string]any{"collection_id": collectionID, "product_id": productID, "assigned": true})
}

func (h *Handler) createProductMedia(w http.ResponseWriter, r *http.Request) {
	var media ProductMedia
	if err := json.NewDecoder(r.Body).Decode(&media); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	media.TenantID = middleware.TenantIDFromContext(r.Context())
	media.RegionID = middleware.RegionIDFromContext(r.Context())
	media.ProductID = chi.URLParam(r, "productID")
	if media.ID == "" {
		media.ID = utils.NewID("med")
	}
	saved, err := h.svc.SaveProductMedia(r.Context(), media)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listProductMedia(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	items, err := h.svc.ListProductMedia(r.Context(), tenantID, regionID, productID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listProductTypes(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	items, err := h.svc.ListProductTypes(r.Context(), tenantID, regionID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createProductType(w http.ResponseWriter, r *http.Request) {
	var pt ProductType
	if err := json.NewDecoder(r.Body).Decode(&pt); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	pt.TenantID = middleware.TenantIDFromContext(r.Context())
	pt.RegionID = middleware.RegionIDFromContext(r.Context())
	if pt.ID == "" {
		pt.ID = utils.NewID("pty")
	}
	saved, err := h.svc.SaveProductType(r.Context(), pt)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) getProductTypeDetail(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productTypeID := chi.URLParam(r, "productTypeID")
	detail, err := h.svc.GetProductTypeDetail(r.Context(), tenantID, regionID, productTypeID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, detail)
}

func (h *Handler) linkAttributeToProductType(w http.ResponseWriter, r *http.Request) {
	var in LinkAttributeToTypeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productTypeID := chi.URLParam(r, "productTypeID")
	if err := h.svc.LinkAttributeToProductType(r.Context(), tenantID, regionID, productTypeID, in); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, map[string]any{"linked": true})
}

func (h *Handler) unlinkAttributeFromProductType(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productTypeID := chi.URLParam(r, "productTypeID")
	attributeID := chi.URLParam(r, "attributeID")
	if err := h.svc.UnlinkAttributeFromProductType(r.Context(), tenantID, regionID, productTypeID, attributeID); err != nil {
		utils.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCatalogAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	items, err := h.svc.ListCatalogAttributes(r.Context(), tenantID, regionID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createCatalogAttribute(w http.ResponseWriter, r *http.Request) {
	var a CatalogAttribute
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	a.TenantID = middleware.TenantIDFromContext(r.Context())
	a.RegionID = middleware.RegionIDFromContext(r.Context())
	if a.ID == "" {
		a.ID = utils.NewID("atr")
	}
	saved, err := h.svc.SaveCatalogAttribute(r.Context(), a)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusCreated, saved)
}

func (h *Handler) listProductAttributeValues(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	items, err := h.svc.ListProductAttributeValues(r.Context(), tenantID, regionID, productID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"values": items})
}

func (h *Handler) putProductAttributeValues(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Values []AttributeValuePair `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	if err := h.svc.SetProductAttributeValues(r.Context(), tenantID, regionID, productID, body.Values); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listVariantAttributeValues(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	variantID := chi.URLParam(r, "variantID")
	items, err := h.svc.ListVariantAttributeValues(r.Context(), tenantID, regionID, productID, variantID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"values": items})
}

func (h *Handler) putVariantAttributeValues(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Values []AttributeValuePair `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"code": "bad_request", "message": "invalid body"})
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	regionID := middleware.RegionIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	variantID := chi.URLParam(r, "variantID")
	if err := h.svc.SetVariantAttributeValues(r.Context(), tenantID, regionID, productID, variantID, body.Values); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
