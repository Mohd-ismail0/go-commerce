package regions

type Region struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	LocaleCode string `json:"locale_code"`
}
