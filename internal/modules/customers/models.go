package customers

type Customer struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}
