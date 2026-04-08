package identity

import (
	"context"
	"database/sql"
	"strings"
	"time"
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

func (r *Repository) CreateAuthSession(ctx context.Context, sessionID, tenantID, userID, refreshHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO auth_sessions (id, tenant_id, user_id, refresh_token_hash, expires_at, revoked_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,NULL,NOW(),NOW())
`, sessionID, tenantID, userID, refreshHash, expiresAt.UTC())
	return err
}

func (r *Repository) GetActiveSessionByRefreshHash(ctx context.Context, tenantID, refreshHash string) (string, string, time.Time, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, expires_at
FROM auth_sessions
WHERE tenant_id = $1 AND refresh_token_hash = $2 AND revoked_at IS NULL
LIMIT 1
`, tenantID, refreshHash)
	var sessionID, userID string
	var expiresAt time.Time
	err := row.Scan(&sessionID, &userID, &expiresAt)
	return sessionID, userID, expiresAt, err
}

func (r *Repository) RotateSessionRefreshToken(ctx context.Context, sessionID, newRefreshHash string, newExpiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE auth_sessions
SET refresh_token_hash = $2, expires_at = $3, updated_at = NOW()
WHERE id = $1
`, sessionID, newRefreshHash, newExpiresAt.UTC())
	return err
}

func (r *Repository) RevokeSessionByRefreshHash(ctx context.Context, tenantID, refreshHash string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE auth_sessions
SET revoked_at = NOW(), updated_at = NOW()
WHERE tenant_id = $1 AND refresh_token_hash = $2 AND revoked_at IS NULL
`, tenantID, refreshHash)
	return err
}
