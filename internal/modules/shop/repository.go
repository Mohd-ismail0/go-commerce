package shop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbsqlc "rewrite/internal/shared/db/sqlc"
)

func settingsScope(tenantID, regionID string) string {
	return "shop.settings:" + tenantID + ":" + regionID
}

var ErrShopSettingsIdempotencyKeyRequired = errors.New("shop settings idempotency key is required")
var ErrShopSettingsIdempotencyMismatch = errors.New("shop settings idempotency record mismatch")
var ErrShopSettingsIdempotencyOrphan = errors.New("shop settings idempotency references missing row")

type Repository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, queries: dbsqlc.New(db)}
}

func (r *Repository) Get(ctx context.Context, tenantID, regionID string) (Settings, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT tenant_id, region_id, display_name, domain, support_email, company_address, metadata::text, updated_at
FROM shop_settings WHERE tenant_id = $1 AND region_id = $2
`, tenantID, regionID)
	s, err := scanSettingsRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{
			TenantID: tenantID, RegionID: regionID,
			Metadata: json.RawMessage(`{}`),
		}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (r *Repository) Patch(ctx context.Context, tenantID, regionID string, patch PatchInput, idempotencyKey string) (Settings, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Settings{}, ErrShopSettingsIdempotencyKeyRequired
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := tenantID + "\x00" + settingsScope(tenantID, regionID) + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Settings{}, err
	}
	qtx := r.queries.WithTx(tx)
	scope := settingsScope(tenantID, regionID)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		if resourceID != "shop_settings" {
			return Settings{}, ErrShopSettingsIdempotencyMismatch
		}
		row := tx.QueryRowContext(ctx, `
SELECT tenant_id, region_id, display_name, domain, support_email, company_address, metadata::text, updated_at
FROM shop_settings WHERE tenant_id = $1 AND region_id = $2
`, tenantID, regionID)
		s, scanErr := scanSettingsRow(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Settings{}, fmt.Errorf("%w: %s/%s", ErrShopSettingsIdempotencyOrphan, tenantID, regionID)
		}
		if scanErr != nil {
			return Settings{}, scanErr
		}
		if err := tx.Commit(); err != nil {
			return Settings{}, err
		}
		return s, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Settings{}, idemErr
	}

	cur, err := loadSettingsTx(ctx, tx, tenantID, regionID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Settings{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		cur = Settings{TenantID: tenantID, RegionID: regionID, Metadata: json.RawMessage(`{}`)}
	}
	merged := mergePatch(cur, patch)
	meta := string(merged.Metadata)
	if meta == "" {
		meta = "{}"
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO shop_settings (tenant_id, region_id, display_name, domain, support_email, company_address, metadata, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,NOW())
ON CONFLICT (tenant_id, region_id) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  domain = EXCLUDED.domain,
  support_email = EXCLUDED.support_email,
  company_address = EXCLUDED.company_address,
  metadata = EXCLUDED.metadata,
  updated_at = NOW()
`, merged.TenantID, merged.RegionID, merged.DisplayName, merged.Domain, merged.SupportEmail, merged.CompanyAddress, meta)
	if err != nil {
		return Settings{}, err
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          scope,
		IdempotencyKey: key,
		ResourceID:     "shop_settings",
	}); err != nil {
		return Settings{}, err
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, err
	}
	return r.Get(ctx, tenantID, regionID)
}

func loadSettingsTx(ctx context.Context, tx *sql.Tx, tenantID, regionID string) (Settings, error) {
	row := tx.QueryRowContext(ctx, `
SELECT tenant_id, region_id, display_name, domain, support_email, company_address, metadata::text, updated_at
FROM shop_settings WHERE tenant_id = $1 AND region_id = $2
`, tenantID, regionID)
	return scanSettingsRow(row)
}

func scanSettingsRow(row *sql.Row) (Settings, error) {
	var s Settings
	var meta sql.NullString
	var updatedAt time.Time
	err := row.Scan(&s.TenantID, &s.RegionID, &s.DisplayName, &s.Domain, &s.SupportEmail, &s.CompanyAddress, &meta, &updatedAt)
	if err != nil {
		return Settings{}, err
	}
	if meta.Valid && strings.TrimSpace(meta.String) != "" {
		s.Metadata = json.RawMessage(meta.String)
	} else {
		s.Metadata = json.RawMessage(`{}`)
	}
	s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return s, nil
}

func mergePatch(cur Settings, p PatchInput) Settings {
	out := cur
	if p.DisplayName != nil {
		out.DisplayName = *p.DisplayName
	}
	if p.Domain != nil {
		out.Domain = *p.Domain
	}
	if p.SupportEmail != nil {
		out.SupportEmail = *p.SupportEmail
	}
	if p.CompanyAddress != nil {
		out.CompanyAddress = *p.CompanyAddress
	}
	if p.Metadata != nil {
		out.Metadata = *p.Metadata
	}
	return out
}
