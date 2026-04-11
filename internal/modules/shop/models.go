package shop

import "encoding/json"

// Settings is tenant/region storefront metadata (Saleor Shop subset).
type Settings struct {
	TenantID       string          `json:"tenant_id"`
	RegionID       string          `json:"region_id"`
	DisplayName    string          `json:"display_name"`
	Domain         string          `json:"domain"`
	SupportEmail   string          `json:"support_email"`
	CompanyAddress string          `json:"company_address"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	UpdatedAt      string          `json:"updated_at,omitempty"`
}

// PatchInput applies a partial update; nil fields are left unchanged.
type PatchInput struct {
	DisplayName    *string          `json:"display_name"`
	Domain         *string          `json:"domain"`
	SupportEmail   *string          `json:"support_email"`
	CompanyAddress *string          `json:"company_address"`
	Metadata       *json.RawMessage `json:"metadata"`
}
