package promotions

type Promotion struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ValueCents int64  `json:"value_cents"`
}
