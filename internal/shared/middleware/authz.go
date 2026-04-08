package middleware

import (
	"database/sql"
	"net/http"
	"strings"

	"rewrite/internal/shared/permissions"
	"rewrite/internal/shared/utils"
)

// PolicyRule maps the longest matching path prefix to a required permission code.
type PolicyRule struct {
	Prefix         string
	PermissionCode string
}

// PolicyAuthorization enforces permission codes on sensitive routes. Access is allowed when:
//   - X-Role is "admin" (case-insensitive), or
//   - X-User-ID is set and the user has the required permission in the database, or
//   - legacy: X-Role is any other non-empty value (same escape hatch as the previous header-only guard).
func PolicyAuthorization(db *sql.DB, rules []PolicyRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			code := MatchPolicyRule(path, rules)
			if code == "" {
				next.ServeHTTP(w, r)
				return
			}

			role := strings.TrimSpace(r.Header.Get("X-Role"))
			if strings.EqualFold(role, "admin") {
				next.ServeHTTP(w, r)
				return
			}

			userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
			if userID != "" {
				tenantID := TenantIDFromContext(r.Context())
				ok, err := permissions.UserHasPermission(r.Context(), db, tenantID, userID, code)
				if err != nil {
					utils.JSON(w, http.StatusInternalServerError, map[string]any{
						"code":    "internal",
						"message": "failed to evaluate permissions",
					})
					return
				}
				if ok {
					next.ServeHTTP(w, r)
					return
				}
				utils.JSON(w, http.StatusForbidden, map[string]any{
					"code":    "forbidden",
					"message": "missing required permission",
				})
				return
			}

			if role != "" {
				next.ServeHTTP(w, r)
				return
			}

			utils.JSON(w, http.StatusForbidden, map[string]any{
				"code":    "forbidden",
				"message": "missing identity: set X-User-ID with a permitted user or X-Role",
			})
		})
	}
}

// MatchPolicyRule returns the permission code for the longest matching prefix rule.
func MatchPolicyRule(path string, rules []PolicyRule) string {
	var best string
	var bestLen int
	for _, rule := range rules {
		p := rule.Prefix
		if p == "" {
			continue
		}
		if strings.HasPrefix(path, p) && len(p) > bestLen {
			bestLen = len(p)
			best = rule.PermissionCode
		}
	}
	return best
}
