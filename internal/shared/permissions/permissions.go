package permissions

import (
	"context"
	"database/sql"
	"errors"
)

// UserHasPermission returns true when the user has the permission code via any role in the tenant.
func UserHasPermission(ctx context.Context, db *sql.DB, tenantID, userID, code string) (bool, error) {
	if tenantID == "" || userID == "" || code == "" {
		return false, nil
	}
	var exists int
	err := db.QueryRowContext(ctx, `
SELECT 1
FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
JOIN user_roles ur ON ur.role_id = rp.role_id
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1 AND r.tenant_id = $2 AND p.code = $3
LIMIT 1
`, userID, tenantID, code).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
