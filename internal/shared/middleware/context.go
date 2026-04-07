package middleware

import "context"

type contextKey string

const (
	tenantIDKey contextKey = "tenant_id"
	regionIDKey contextKey = "region_id"
)

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

func TenantIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(tenantIDKey).(string)
	return value
}

func WithRegionID(ctx context.Context, regionID string) context.Context {
	return context.WithValue(ctx, regionIDKey, regionID)
}

func RegionIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(regionIDKey).(string)
	return value
}
