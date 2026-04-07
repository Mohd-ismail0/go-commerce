package payments

type Payment struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	RegionID          string `json:"region_id"`
	OrderID           string `json:"order_id,omitempty"`
	CheckoutID        string `json:"checkout_id,omitempty"`
	Provider          string `json:"provider"`
	Status            string `json:"status"`
	AmountCents       int64  `json:"amount_cents"`
	Currency          string `json:"currency"`
	ExternalReference string `json:"external_reference,omitempty"`
}
