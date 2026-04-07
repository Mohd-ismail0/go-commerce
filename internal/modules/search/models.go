package search

type Document struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Query      string `json:"query"`
}
