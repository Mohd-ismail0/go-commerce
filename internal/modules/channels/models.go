package channels

type Channel struct {
	ID              string `json:"id"`
	TenantID        string `json:"tenant_id"`
	RegionID        string `json:"region_id"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	DefaultCurrency string `json:"default_currency"`
	DefaultCountry  string `json:"default_country"`
	IsActive        bool   `json:"is_active"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}
