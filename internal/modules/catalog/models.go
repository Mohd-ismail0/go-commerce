package catalog

type Product struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	PriceCents int64  `json:"price_cents"`
}
