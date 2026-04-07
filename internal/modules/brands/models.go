package brands

type Brand struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	RegionID       string `json:"region_id"`
	Name           string `json:"name"`
	DefaultLocale  string `json:"default_locale"`
	DefaultCurrency string `json:"default_currency"`
}
