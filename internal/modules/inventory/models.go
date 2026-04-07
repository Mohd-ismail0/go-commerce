package inventory

type StockItem struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}
