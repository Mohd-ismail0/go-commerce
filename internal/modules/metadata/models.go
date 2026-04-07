package metadata

type Entry struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	RegionID   string         `json:"region_id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	IsPrivate  bool           `json:"is_private"`
	Metadata   map[string]any `json:"metadata"`
}
