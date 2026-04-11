package shipping

type ShippingZone struct {
	ID        string   `json:"id"`
	TenantID  string   `json:"tenant_id"`
	RegionID  string   `json:"region_id"`
	Name      string   `json:"name"`
	Countries []string `json:"countries"`
}

type ShippingMethod struct {
	ID                        string   `json:"id"`
	TenantID                  string   `json:"tenant_id"`
	RegionID                  string   `json:"region_id"`
	ShippingZoneID            string   `json:"shipping_zone_id"`
	Name                      string   `json:"name"`
	PriceCents                int64    `json:"price_cents"`
	Currency                  string   `json:"currency"`
	MinOrderCents             *int64   `json:"min_order_cents,omitempty"`
	MaxOrderCents             *int64   `json:"max_order_cents,omitempty"`
	ChannelIDs                []string `json:"channel_ids,omitempty"`
	PostalPrefixes            []string `json:"postal_prefixes,omitempty"`
	DeliveryDaysMin           *int64   `json:"delivery_days_min,omitempty"`
	DeliveryDaysMax           *int64   `json:"delivery_days_max,omitempty"`
	Description               string   `json:"description,omitempty"`
	WeightSurchargePerKgCents *int64   `json:"weight_surcharge_per_kg_cents,omitempty"`
	// QuotedPriceCents is set only by POST /shipping/resolve when weight-based quoting applies.
	QuotedPriceCents *int64 `json:"quoted_price_cents,omitempty"`
}

// ResolveInput selects applicable shipping methods for a cart/checkout.
type ResolveInput struct {
	CountryCode      string `json:"country_code"`
	ChannelID        string `json:"channel_id"`
	PostalCode       string `json:"postal_code"`
	OrderTotalCents  int64  `json:"order_total_cents"`
	Currency         string `json:"currency"`
	TotalWeightGrams *int64 `json:"total_weight_grams,omitempty"`
}
