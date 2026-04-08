package search

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Document) Document {
	_, _ = r.db.Exec(`
INSERT INTO search_documents (id, tenant_id, region_id, entity_type, entity_id, document, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,to_tsvector('simple', $6),NOW(),NOW())
ON CONFLICT (tenant_id, region_id, entity_type, entity_id) DO UPDATE SET
document = EXCLUDED.document,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.EntityType, item.EntityID, item.Query)
	return item
}

func (r *Repository) Query(tenantID, regionID, entityType, query string, limit int) []SearchHit {
	rows, err := r.db.Query(`
SELECT entity_type, entity_id
FROM search_documents
WHERE tenant_id = $1
  AND region_id = $2
  AND ($3::text = '' OR entity_type = $3)
  AND ($4::text = '' OR document @@ plainto_tsquery('simple', $4))
ORDER BY updated_at DESC
LIMIT $5
`, tenantID, regionID, entityType, query, limit)
	if err != nil {
		return []SearchHit{}
	}
	defer rows.Close()
	out := make([]SearchHit, 0, limit)
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.EntityType, &h.EntityID); err != nil {
			continue
		}
		out = append(out, h)
	}
	return out
}
