package identity

type User struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Email    string `json:"email"`
	IsStaff  bool   `json:"is_staff"`
	IsActive bool   `json:"is_active"`
}
