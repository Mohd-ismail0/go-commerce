package localization

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

func (r *Repository) Save(item Translation) Translation {
	b, _ := json.Marshal(item.Fields)
	_, _ = r.db.Exec(`
INSERT INTO translations (id, tenant_id, region_id, entity_type, entity_id, language_code, fields, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,NOW(),NOW())
ON CONFLICT (tenant_id, region_id, entity_type, entity_id, language_code) DO UPDATE SET
fields = EXCLUDED.fields,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.EntityType, item.EntityID, item.LanguageCode, string(b))
	return item
}
