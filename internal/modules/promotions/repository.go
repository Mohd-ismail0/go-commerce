package promotions

import (
	"database/sql"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(item Promotion) Promotion {
	_, _ = r.db.Exec(`
INSERT INTO promotions (id, tenant_id, region_id, name, kind, value_cents, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
name = EXCLUDED.name,
kind = EXCLUDED.kind,
value_cents = EXCLUDED.value_cents,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Name, item.Kind, item.ValueCents)
	return item
}

func (r *Repository) List(tenantID string) []Promotion {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, name, kind, value_cents FROM promotions
WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Promotion{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Promotion{}
	for rows.Next() {
		var p Promotion
		if err := rows.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Name, &p.Kind, &p.ValueCents); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (r *Repository) SaveRule(item PromotionRule) PromotionRule {
	_, _ = r.db.Exec(`
INSERT INTO promotion_rules (id, tenant_id, region_id, promotion_id, rule_type, value_cents, currency, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
promotion_id = EXCLUDED.promotion_id,
rule_type = EXCLUDED.rule_type,
value_cents = EXCLUDED.value_cents,
currency = EXCLUDED.currency,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.PromotionID, item.RuleType, item.ValueCents, item.Currency)
	return item
}

func (r *Repository) ListRules(tenantID string) []PromotionRule {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, promotion_id, rule_type, value_cents, currency
FROM promotion_rules
WHERE tenant_id = $1
ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []PromotionRule{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []PromotionRule{}
	for rows.Next() {
		var p PromotionRule
		if err := rows.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.PromotionID, &p.RuleType, &p.ValueCents, &p.Currency); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (r *Repository) SaveVoucher(item Voucher) Voucher {
	var startsAt any
	var endsAt any
	if ts, err := parseRFC3339OrZero(item.StartsAt); err == nil && !ts.IsZero() {
		startsAt = ts.UTC()
	}
	if ts, err := parseRFC3339OrZero(item.EndsAt); err == nil && !ts.IsZero() {
		endsAt = ts.UTC()
	}
	_, _ = r.db.Exec(`
INSERT INTO vouchers (id, tenant_id, region_id, code, discount_type, value_cents, currency, usage_limit, used_count, starts_at, ends_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, 0), $10, $11, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
code = EXCLUDED.code,
discount_type = EXCLUDED.discount_type,
value_cents = EXCLUDED.value_cents,
currency = EXCLUDED.currency,
usage_limit = EXCLUDED.usage_limit,
starts_at = EXCLUDED.starts_at,
ends_at = EXCLUDED.ends_at,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.Code, item.DiscountType, item.ValueCents, item.Currency, item.UsageLimit, nil, startsAt, endsAt)
	return item
}

func (r *Repository) ListVouchers(tenantID string) []Voucher {
	rows, err := r.db.Query(`
SELECT id, tenant_id, region_id, code, discount_type, value_cents, currency, usage_limit, used_count, starts_at, ends_at
FROM vouchers
WHERE tenant_id = $1
ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return []Voucher{}
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Voucher{}
	for rows.Next() {
		var v Voucher
		var startsAt sql.NullTime
		var endsAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.TenantID, &v.RegionID, &v.Code, &v.DiscountType, &v.ValueCents, &v.Currency, &v.UsageLimit, &v.UsedCount, &startsAt, &endsAt); err == nil {
			if startsAt.Valid {
				v.StartsAt = startsAt.Time.UTC().Format(time.RFC3339)
			}
			if endsAt.Valid {
				v.EndsAt = endsAt.Time.UTC().Format(time.RFC3339)
			}
			out = append(out, v)
		}
	}
	return out
}

func (r *Repository) GetPromotionByID(id, tenantID, regionID string) (Promotion, bool) {
	var p Promotion
	row := r.db.QueryRow(`
SELECT id, tenant_id, region_id, name, kind, value_cents
FROM promotions
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, id, tenantID, regionID)
	if err := row.Scan(&p.ID, &p.TenantID, &p.RegionID, &p.Name, &p.Kind, &p.ValueCents); err != nil {
		return Promotion{}, false
	}
	return p, true
}

func (r *Repository) FindEligibleVoucher(tenantID, regionID, code, currency string, at time.Time) (Voucher, bool) {
	row := r.db.QueryRow(`
SELECT id, tenant_id, region_id, code, discount_type, value_cents, currency, usage_limit, used_count, starts_at, ends_at
FROM vouchers
WHERE tenant_id = $1
  AND region_id = $2
  AND code = $3
  AND currency = $4
  AND (starts_at IS NULL OR starts_at <= $5)
  AND (ends_at IS NULL OR ends_at >= $5)
  AND (usage_limit IS NULL OR used_count < usage_limit)
LIMIT 1
`, tenantID, regionID, code, currency, at.UTC())
	return scanVoucher(row)
}

func (r *Repository) ConsumeVoucherByID(voucherID string) bool {
	row := r.db.QueryRow(`
UPDATE vouchers
SET used_count = used_count + 1, updated_at = NOW()
WHERE id = $1
  AND (usage_limit IS NULL OR used_count < usage_limit)
RETURNING id, tenant_id, region_id, code, discount_type, value_cents, currency, usage_limit, used_count, starts_at, ends_at
`, voucherID)
	_, ok := scanVoucher(row)
	return ok
}

func scanVoucher(row *sql.Row) (Voucher, bool) {
	var v Voucher
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	if err := row.Scan(&v.ID, &v.TenantID, &v.RegionID, &v.Code, &v.DiscountType, &v.ValueCents, &v.Currency, &v.UsageLimit, &v.UsedCount, &startsAt, &endsAt); err != nil {
		return Voucher{}, false
	}
	if startsAt.Valid {
		v.StartsAt = startsAt.Time.UTC().Format(time.RFC3339)
	}
	if endsAt.Valid {
		v.EndsAt = endsAt.Time.UTC().Format(time.RFC3339)
	}
	return v, true
}

func parseRFC3339OrZero(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}
