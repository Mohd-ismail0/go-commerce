package catalog

import "encoding/json"

type Product struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Slug       string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	SEOTitle   string `json:"seo_title,omitempty"`
	SEODescription string `json:"seo_description,omitempty"`
	Metadata   string `json:"metadata,omitempty"`
	ExternalReference string `json:"external_reference,omitempty"`
	Currency   string `json:"currency"`
	PriceCents int64  `json:"price_cents"`
	ProductTypeID string `json:"product_type_id,omitempty"`
	AttributeValues []AttributeValuePair `json:"attribute_values,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

func (p Product) GetTenantID() string {
	return p.TenantID
}

func (p Product) GetRegionID() string {
	return p.RegionID
}

type ProductVariant struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	ProductID  string `json:"product_id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	PriceCents int64  `json:"price_cents"`
}

type Category struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ParentID string `json:"parent_id,omitempty"`
}

type Collection struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
}

type ProductMedia struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	ProductID string `json:"product_id"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
}

// AttributeValuePair is a stored product- or variant-level attribute value.
type AttributeValuePair struct {
	AttributeID string `json:"attribute_id"`
	Value       string `json:"value"`
}

// ProductType is a reusable template of attributes (Saleor-style product type).
type ProductType struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"created_at,omitempty"`
}

// CatalogAttribute defines an attribute (name, input validation rules).
type CatalogAttribute struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	RegionID      string          `json:"region_id"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	InputType     string          `json:"input_type"`
	Unit          string          `json:"unit,omitempty"`
	AllowedValues json.RawMessage `json:"allowed_values,omitempty"`
	CreatedAt     string          `json:"created_at,omitempty"`
}

// ProductTypeAttributeDef is an attribute assigned to a product type.
type ProductTypeAttributeDef struct {
	AttributeID   string          `json:"attribute_id"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	InputType     string          `json:"input_type"`
	Unit          string          `json:"unit,omitempty"`
	SortOrder     int             `json:"sort_order"`
	VariantOnly   bool            `json:"variant_only"`
	AllowedValues json.RawMessage `json:"allowed_values,omitempty"`
}

// ProductTypeDetail is a product type with its attribute schema.
type ProductTypeDetail struct {
	ProductType
	Attributes []ProductTypeAttributeDef `json:"attributes"`
}

// LinkAttributeToTypeInput links a catalog attribute to a product type.
type LinkAttributeToTypeInput struct {
	AttributeID string `json:"attribute_id"`
	SortOrder   int    `json:"sort_order"`
	VariantOnly bool   `json:"variant_only"`
}
