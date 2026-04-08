package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"rewrite/internal/shared/permissions"
	"rewrite/internal/shared/utils"
)

// PolicyRule maps the longest matching path prefix to a required permission code.
type PolicyRule struct {
	Prefix         string
	PermissionCode string
}

type PolicyOptions struct {
	UserJWTSecret         string
	UserJWTKeys           []JWTKey
	AllowLegacyRoleBypass bool
}

// PolicyAuthorization enforces permission codes on sensitive routes. Access is allowed when:
//   - X-User-JWT is valid and has role "admin", or
//   - X-User-JWT is valid and mapped user has required permission in DB.
//
// Optional compatibility mode: AllowLegacyRoleBypass permits legacy non-empty X-Role values.
func PolicyAuthorization(db *sql.DB, rules []PolicyRule, opts PolicyOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			code := MatchPolicyRule(path, rules)
			if code == "" {
				next.ServeHTTP(w, r)
				return
			}

			userID, roles, jwtErr := resolveIdentity(r, opts)
			if jwtErr != nil {
				utils.JSON(w, http.StatusUnauthorized, map[string]any{
					"code":    "unauthorized",
					"message": jwtErr.Error(),
				})
				return
			}
			if strings.TrimSpace(userID) == "" {
				utils.JSON(w, http.StatusUnauthorized, map[string]any{
					"code":    "unauthorized",
					"message": "missing user authentication token",
				})
				return
			}
			if hasAdminRole(roles) {
				next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
				return
			}

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
				next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
				return
			}

			utils.JSON(w, http.StatusForbidden, map[string]any{
				"code":    "forbidden",
				"message": "missing required permission",
			})
		})
	}
}

func resolveIdentity(r *http.Request, opts PolicyOptions) (string, []string, error) {
	token := UserJWTFromRequest(r)
	if token != "" {
		secret := strings.TrimSpace(opts.UserJWTSecret)
		if secret == "" {
			return "", nil, httpError("user jwt secret is not configured")
		}
		var claims UserClaims
		var err error
		if len(opts.UserJWTKeys) > 0 {
			claims, err = ParseAndVerifyUserJWTWithKeys(token, opts.UserJWTKeys, time.Now().UTC())
		} else {
			claims, err = ParseAndVerifyUserJWT(token, secret, time.Now().UTC())
		}
		if err != nil {
			return "", nil, httpError(err.Error())
		}
		reqTenant := TenantIDFromContext(r.Context())
		if claims.TenantID != "" && reqTenant != "" && !strings.EqualFold(claims.TenantID, reqTenant) {
			return "", nil, httpError("jwt tenant mismatch")
		}
		return claims.Subject, claims.Roles, nil
	}

	if opts.AllowLegacyRoleBypass {
		role := strings.TrimSpace(r.Header.Get("X-Role"))
		if role != "" {
			userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
			if userID == "" {
				userID = "legacy-role-user"
			}
			return userID, []string{role}, nil
		}
	}
	return "", nil, nil
}

// UserJWTFromRequest supports both legacy X-User-JWT and standard Authorization Bearer.
func UserJWTFromRequest(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-User-JWT"))
	if token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func hasAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "admin") {
			return true
		}
	}
	return false
}

type httpError string

func (e httpError) Error() string { return string(e) }

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
