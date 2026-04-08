package catalog

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
