package apps

type App struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	RegionID  string `json:"region_id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	AuthToken string `json:"auth_token,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
