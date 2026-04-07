package checkout

type Session struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	RegionID      string `json:"region_id"`
	CustomerID    string `json:"customer_id"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	SubtotalCents int64  `json:"subtotal_cents"`
	ShippingCents int64  `json:"shipping_cents"`
	TaxCents      int64  `json:"tax_cents"`
	TotalCents    int64  `json:"total_cents"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type Line struct {
	ID             string `json:"id"`
	CheckoutID     string `json:"checkout_id"`
	ProductID      string `json:"product_id,omitempty"`
	VariantID      string `json:"variant_id,omitempty"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Currency       string `json:"currency"`
}

type CompleteResult struct {
	OrderID string `json:"order_id"`
}

type OrderCreatedPayload struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
	TotalCents int64  `json:"total_cents"`
	Currency   string `json:"currency"`
	CheckoutID string `json:"checkout_id"`
}

func (o OrderCreatedPayload) GetTenantID() string {
	return o.TenantID
}

func (o OrderCreatedPayload) GetRegionID() string {
	return o.RegionID
}
