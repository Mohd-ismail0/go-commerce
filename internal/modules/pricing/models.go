package pricing

type PriceBookEntry struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	ProductID string `json:"product_id"`
	Currency  string `json:"currency"`
	AmountCents int64 `json:"amount_cents"`
}
