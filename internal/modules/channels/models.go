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

// ProductChannelListing links a catalog product to a sales channel (Saleor ProductChannelListing).
type ProductChannelListing struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	RegionID          string `json:"region_id"`
	ProductID         string `json:"product_id"`
	ChannelID         string `json:"channel_id"`
	IsPublished       bool   `json:"is_published"`
	VisibleInListings bool   `json:"visible_in_listings"`
	PublishedAt       string `json:"published_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}
