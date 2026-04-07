package fulfillments

type Fulfillment struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	RegionID       string `json:"region_id"`
	OrderID        string `json:"order_id"`
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}
