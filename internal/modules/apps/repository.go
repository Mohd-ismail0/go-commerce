package apps

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, in App) (App, error) {
	var out App
	err := r.db.QueryRowContext(ctx, `
INSERT INTO apps (id, tenant_id, region_id, name, is_active, auth_token, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
is_active = EXCLUDED.is_active,
auth_token = EXCLUDED.auth_token,
updated_at = NOW()
RETURNING id, tenant_id, region_id, name, is_active, COALESCE(auth_token,''), updated_at
`, in.ID, in.TenantID, in.RegionID, in.Name, in.IsActive, nullString(in.AuthToken)).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.Name, &out.IsActive, &out.AuthToken, &out.UpdatedAt,
	)
	return out, err
}

func (r *Repository) GetByID(ctx context.Context, tenantID, regionID, id string) (App, bool, error) {
	var out App
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, name, is_active, COALESCE(auth_token,''), updated_at
FROM apps
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, id, tenantID, regionID).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.Name, &out.IsActive, &out.AuthToken, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return App{}, false, nil
	}
	if err != nil {
		return App{}, false, err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return out, true, nil
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, name, is_active, COALESCE(auth_token,''), updated_at
FROM apps
WHERE tenant_id = $1
  AND region_id = $2
  AND ($3::bool = false OR is_active = true)
ORDER BY updated_at DESC
`, tenantID, regionID, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []App{}
	for rows.Next() {
		var item App
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.Name, &item.IsActive, &item.AuthToken, &updatedAt); err != nil {
			return nil, err
		}
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}
