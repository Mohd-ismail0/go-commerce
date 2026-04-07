package identity

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item User) User {
	_, _ = r.db.Exec(`
INSERT INTO users (id, tenant_id, region_id, email, password_hash, is_staff, is_active, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
email = EXCLUDED.email,
is_staff = EXCLUDED.is_staff,
is_active = EXCLUDED.is_active,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Email, "placeholder", item.IsStaff, item.IsActive)
	return item
}

func (r *Repository) List(tenantID string) []User {
	rows, err := r.db.Query(`SELECT id, tenant_id, region_id, email, is_staff, is_active FROM users WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return []User{}
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.Email, &item.IsStaff, &item.IsActive); err == nil {
			out = append(out, item)
		}
	}
	return out
}
