package metadata

import (
	"database/sql"
	"encoding/json"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Entry) Entry {
	b, _ := json.Marshal(item.Metadata)
	_, _ = r.db.Exec(`
INSERT INTO entity_metadata (id, tenant_id, region_id, entity_type, entity_id, is_private, metadata, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,NOW(),NOW())
ON CONFLICT (tenant_id, region_id, entity_type, entity_id, is_private) DO UPDATE SET
metadata = EXCLUDED.metadata,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.EntityType, item.EntityID, item.IsPrivate, string(b))
	return item
}
