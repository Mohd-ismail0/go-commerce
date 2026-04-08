package middleware

import (
	"net/http"
	"strings"

	"rewrite/internal/shared/utils"
)

type RoleValidator func(role string, r *http.Request) bool

func APIToken(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicRoute(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if strings.TrimSpace(expectedToken) == "" {
				utils.JSON(w, http.StatusServiceUnavailable, map[string]any{
					"code":    "auth_unavailable",
					"message": "api authentication is not configured",
				})
				return
			}

			providedToken := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(providedToken), "bearer ") {
				providedToken = strings.TrimSpace(providedToken[7:])
			}
			if providedToken == "" {
				providedToken = strings.TrimSpace(r.Header.Get("X-API-Token"))
			}

			if providedToken == "" || providedToken != expectedToken {
				utils.JSON(w, http.StatusUnauthorized, map[string]any{
					"code":    "unauthorized",
					"message": "invalid or missing api token",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RoleGuard(hooks map[string]RoleValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if !requiresRole(path, hooks) {
				next.ServeHTTP(w, r)
				return
			}

			role := strings.TrimSpace(r.Header.Get("X-Role"))
			if role == "" {
				utils.JSON(w, http.StatusForbidden, map[string]any{
					"code":    "forbidden",
					"message": "missing required role",
				})
				return
			}

			validator := MatchRoleValidator(path, hooks)
			if validator != nil && !validator(role, r) {
				utils.JSON(w, http.StatusForbidden, map[string]any{
					"code":    "forbidden",
					"message": "insufficient role",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func DefaultRoleValidator(role string, _ *http.Request) bool {
	return strings.TrimSpace(role) != ""
}

func isHealthRoute(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/healthz" || path == "/readyz"
}

// isPublicRoute includes health checks and provider webhook callbacks (verified separately).
func isPublicRoute(path string) bool {
	path = strings.TrimSpace(path)
	if isHealthRoute(path) {
		return true
	}
	if path == "/identity/auth/login" {
		return true
	}
	return strings.HasPrefix(path, "/webhooks/")
}

func requiresRole(path string, hooks map[string]RoleValidator) bool {
	return MatchRoleValidator(path, hooks) != nil
}

func MatchRoleValidator(path string, hooks map[string]RoleValidator) RoleValidator {
	for prefix, validator := range hooks {
		if strings.HasPrefix(path, prefix) {
			return validator
		}
	}
	return nil
}
