package giftcard

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Repository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string) ([]Card, error) {
	rows, err := r.DB.QueryContext(ctx, `
SELECT id, tenant_id, region_id, code, balance_cents, currency, is_active,
       expires_at, created_at, updated_at
FROM gift_cards
WHERE tenant_id = $1 AND region_id = $2
ORDER BY created_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Card
	for rows.Next() {
		var c Card
		var exp, ca, ua sql.NullTime
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Code, &c.BalanceCents, &c.Currency, &c.IsActive, &exp, &ca, &ua); err != nil {
			return nil, err
		}
		if exp.Valid {
			c.ExpiresAt = exp.Time.UTC().Format(time.RFC3339Nano)
		}
		if ca.Valid {
			c.CreatedAt = ca.Time.UTC().Format(time.RFC3339Nano)
		}
		if ua.Valid {
			c.UpdatedAt = ua.Time.UTC().Format(time.RFC3339Nano)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) InsertTx(ctx context.Context, tx *sql.Tx, c Card) (Card, error) {
	var exp interface{}
	if strings.TrimSpace(c.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil {
			return Card{}, err
		}
		exp = t.UTC()
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO gift_cards (id, tenant_id, region_id, code, balance_cents, currency, is_active, expires_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
RETURNING id, tenant_id, region_id, code, balance_cents, currency, is_active, expires_at, created_at, updated_at
`, c.ID, c.TenantID, c.RegionID, c.Code, c.BalanceCents, c.Currency, c.IsActive, exp)
	return scanCard(row)
}

func scanCard(row *sql.Row) (Card, error) {
	var c Card
	var exp, ca, ua sql.NullTime
	if err := row.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Code, &c.BalanceCents, &c.Currency, &c.IsActive, &exp, &ca, &ua); err != nil {
		return Card{}, err
	}
	if exp.Valid {
		c.ExpiresAt = exp.Time.UTC().Format(time.RFC3339Nano)
	}
	if ca.Valid {
		c.CreatedAt = ca.Time.UTC().Format(time.RFC3339Nano)
	}
	if ua.Valid {
		c.UpdatedAt = ua.Time.UTC().Format(time.RFC3339Nano)
	}
	return c, nil
}

var ErrDuplicateCode = errors.New("gift card code already exists for tenant/region")
