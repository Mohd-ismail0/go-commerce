package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"rewrite/internal/shared/utils"
)

var tenantRegionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}$`)

func TenantRegion(defaultTenantID, defaultRegionID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := normalize(strings.TrimSpace(r.Header.Get("X-Tenant-ID")))
			regionID := normalize(strings.TrimSpace(r.Header.Get("X-Region-ID")))

			if tenantID == "" {
				tenantID = normalize(tenantFromHost(r.Host, defaultTenantID))
			}
			if tenantID == "" {
				tenantID = normalize(defaultTenantID)
			}
			if regionID == "" {
				regionID = normalize(defaultRegionID)
			}

			if !isValidKey(tenantID) || !isValidKey(regionID) {
				utils.JSON(w, http.StatusBadRequest, map[string]string{
					"error": "invalid tenant or region identifier",
				})
				return
			}

			ctx := WithTenantID(r.Context(), tenantID)
			ctx = WithRegionID(ctx, regionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isValidKey(value string) bool {
	return tenantRegionPattern.MatchString(value)
}

func tenantFromHost(host, fallback string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return fallback
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 3 {
		return parts[0]
	}
	return fallback
}
