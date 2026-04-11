package giftcard

// Card is a stored-value instrument (Saleor gift card subset).
type Card struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	RegionID     string `json:"region_id"`
	Code         string `json:"code"`
	BalanceCents int64  `json:"balance_cents"`
	Currency     string `json:"currency"`
	IsActive     bool   `json:"is_active"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// CreateInput is used to issue a new card; code is normalized to uppercase in the service.
type CreateInput struct {
	ID              string
	Code            string
	BalanceCents    int64
	Currency        string
	IsActive        bool
	ExpiresAtRFC3339 string // optional, empty = no expiry
}
