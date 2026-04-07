package middleware

import (
	"net/http"
	"strings"
)

func TenantRegion(defaultTenantID, defaultRegionID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			regionID := strings.TrimSpace(r.Header.Get("X-Region-ID"))

			if tenantID == "" {
				tenantID = tenantFromHost(r.Host, defaultTenantID)
			}
			if tenantID == "" {
				tenantID = defaultTenantID
			}
			if regionID == "" {
				regionID = defaultRegionID
			}

			ctx := WithTenantID(r.Context(), tenantID)
			ctx = WithRegionID(ctx, regionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
