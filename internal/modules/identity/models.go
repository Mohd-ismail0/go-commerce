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
	DeviceID string `json:"device_id,omitempty"`
}

type LoginResult struct {
	Token            string   `json:"token"`
	TokenType        string   `json:"token_type"`
	ExpiresAt        int64    `json:"expires_at"`
	RefreshToken     string   `json:"refresh_token"`
	RefreshExpiresAt int64    `json:"refresh_expires_at"`
	UserID           string   `json:"user_id"`
	TenantID         string   `json:"tenant_id"`
	Roles            []string `json:"roles"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id,omitempty"`
}

type SessionInfo struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	DeviceID      string `json:"device_id,omitempty"`
	IPHash        string `json:"ip_hash,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	ExpiresAt     int64  `json:"expires_at"`
	RevokedAt     int64  `json:"revoked_at,omitempty"`
	CompromisedAt int64  `json:"compromised_at,omitempty"`
}
