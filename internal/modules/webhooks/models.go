package webhooks

type Subscription struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	RegionID    string `json:"region_id"`
	AppID       string `json:"app_id,omitempty"`
	EventName   string `json:"event_name"`
	EndpointURL string `json:"endpoint_url"`
	Secret      string `json:"secret,omitempty"`
	IsActive    bool   `json:"is_active"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}
