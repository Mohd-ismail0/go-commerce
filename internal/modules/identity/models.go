package identity

type User struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Email    string `json:"email"`
	IsStaff  bool   `json:"is_staff"`
	IsActive bool   `json:"is_active"`
	Password string `json:"password,omitempty"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResult struct {
	Token     string   `json:"token"`
	TokenType string   `json:"token_type"`
	ExpiresAt int64    `json:"expires_at"`
	UserID    string   `json:"user_id"`
	TenantID  string   `json:"tenant_id"`
	Roles     []string `json:"roles"`
}
