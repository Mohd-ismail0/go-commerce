package shipping

type ShippingMethod struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	RegionID       string `json:"region_id"`
	ShippingZoneID string `json:"shipping_zone_id"`
	Name           string `json:"name"`
	PriceCents     int64  `json:"price_cents"`
	Currency       string `json:"currency"`
}
