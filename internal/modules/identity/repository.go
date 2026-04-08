package identity

import (
	"context"
	"database/sql"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, item User, passwordHash string) (User, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, region_id, email, password_hash, is_staff, is_active, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
email = EXCLUDED.email,
password_hash = CASE WHEN EXCLUDED.password_hash <> '' THEN EXCLUDED.password_hash ELSE users.password_hash END,
is_staff = EXCLUDED.is_staff,
is_active = EXCLUDED.is_active,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, strings.ToLower(strings.TrimSpace(item.Email)), passwordHash, item.IsStaff, item.IsActive)
	if err != nil {
		return User{}, err
	}
	item.Password = ""
	return item, nil
}

func (r *Repository) List(ctx context.Context, tenantID string) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, tenant_id, region_id, email, is_staff, is_active FROM users WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.TenantID, &item.RegionID, &item.Email, &item.IsStaff, &item.IsActive); err == nil {
			out = append(out, item)
		}
	}
	return out, rows.Err()
}

func (r *Repository) GetByEmail(ctx context.Context, tenantID, email string) (User, string, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, email, is_staff, is_active, password_hash
FROM users
WHERE tenant_id = $1 AND lower(email) = $2
LIMIT 1
`, tenantID, strings.ToLower(strings.TrimSpace(email)))
	var user User
	var passwordHash string
	err := row.Scan(&user.ID, &user.TenantID, &user.RegionID, &user.Email, &user.IsStaff, &user.IsActive, &passwordHash)
	return user, passwordHash, err
}

func (r *Repository) RolesForUser(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT r.name
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1 AND r.tenant_id = $2
`, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err == nil {
			out = append(out, role)
		}
	}
	return out, rows.Err()
}
