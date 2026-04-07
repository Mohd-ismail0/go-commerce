package orders

type Order struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	CustomerID string `json:"customer_id"`
	Status    string `json:"status"`
	TotalCents int64 `json:"total_cents"`
	Currency  string `json:"currency"`
}
