package inventory

type StockItem struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id,omitempty"`
	Quantity  int64  `json:"quantity"`
}
