package orders

import "time"

type Order struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	RegionID    string `json:"region_id"`
	CustomerID  string `json:"customer_id"`
	Status      string `json:"status"`
	TotalCents  int64  `json:"total_cents"`
	Currency    string `json:"currency"`
	VoucherCode string `json:"voucher_code,omitempty"`
	PromotionID string `json:"promotion_id,omitempty"`
	TaxClassID  string `json:"tax_class_id,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func (o Order) GetTenantID() string {
	return o.TenantID
}

func (o Order) GetRegionID() string {
	return o.RegionID
}

type StatusUpdateInput struct {
	ID                string
	Status            string
	ExpectedUpdatedAt time.Time
}
