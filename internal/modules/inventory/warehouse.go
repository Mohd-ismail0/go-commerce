package inventory

// Warehouse is a stock location within tenant/region (Saleor warehouse subset).
type Warehouse struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	IsActive bool   `json:"is_active"`
}
