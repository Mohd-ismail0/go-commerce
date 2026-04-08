package shipping

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SaveZone(ctx context.Context, z ShippingZone) (ShippingZone, error) {
	countriesJSON, err := json.Marshal(z.Countries)
	if err != nil {
		return ShippingZone{}, err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO shipping_zones (id, tenant_id, region_id, name, countries, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5::jsonb,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
name = EXCLUDED.name,
countries = EXCLUDED.countries,
updated_at = NOW()
`, z.ID, z.TenantID, z.RegionID, z.Name, countriesJSON)
	if err != nil {
		return ShippingZone{}, err
	}
	return r.GetZoneByID(ctx, z.TenantID, z.ID)
}

func (r *Repository) ListZones(ctx context.Context, tenantID string) ([]ShippingZone, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, name, countries
FROM shipping_zones WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShippingZone{}
	for rows.Next() {
		z, err := scanZone(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

func (r *Repository) GetZoneByID(ctx context.Context, tenantID, id string) (ShippingZone, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, name, countries
FROM shipping_zones WHERE tenant_id = $1 AND id = $2
`, tenantID, id)
	return scanZoneRow(row)
}

func scanZone(rows interface {
	Scan(dest ...any) error
}) (ShippingZone, error) {
	var z ShippingZone
	var raw []byte
	err := rows.Scan(&z.ID, &z.TenantID, &z.RegionID, &z.Name, &raw)
	if err != nil {
		return ShippingZone{}, err
	}
	_ = json.Unmarshal(raw, &z.Countries)
	return z, nil
}

func scanZoneRow(row *sql.Row) (ShippingZone, error) {
	var z ShippingZone
	var raw []byte
	err := row.Scan(&z.ID, &z.TenantID, &z.RegionID, &z.Name, &raw)
	if err != nil {
		return ShippingZone{}, err
	}
	_ = json.Unmarshal(raw, &z.Countries)
	return z, nil
}

func (r *Repository) SaveMethod(ctx context.Context, item ShippingMethod) (ShippingMethod, error) {
	chJSON, err := json.Marshal(item.ChannelIDs)
	if err != nil {
		return ShippingMethod{}, err
	}
	postJSON, err := json.Marshal(item.PostalPrefixes)
	if err != nil {
		return ShippingMethod{}, err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO shipping_methods (id, tenant_id, region_id, shipping_zone_id, name, price_cents, currency, min_order_cents, max_order_cents, channel_ids, postal_prefixes, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,NOW(),NOW())
ON CONFLICT (id) DO UPDATE SET
name = EXCLUDED.name,
price_cents = EXCLUDED.price_cents,
currency = EXCLUDED.currency,
min_order_cents = EXCLUDED.min_order_cents,
max_order_cents = EXCLUDED.max_order_cents,
channel_ids = EXCLUDED.channel_ids,
postal_prefixes = EXCLUDED.postal_prefixes,
updated_at = NOW()
`, item.ID, item.TenantID, item.RegionID, item.ShippingZoneID, item.Name, item.PriceCents, item.Currency,
		nullableInt64(item.MinOrderCents), nullableInt64(item.MaxOrderCents), chJSON, postJSON)
	if err != nil {
		return ShippingMethod{}, err
	}
	return r.GetMethodByID(ctx, item.TenantID, item.ID)
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func (r *Repository) ListMethods(ctx context.Context, tenantID string) ([]ShippingMethod, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, shipping_zone_id, name, price_cents, currency,
       min_order_cents, max_order_cents, channel_ids, postal_prefixes
FROM shipping_methods WHERE tenant_id = $1 ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShippingMethod{}
	for rows.Next() {
		m, err := scanMethod(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) ListMethodsForTenantRegion(ctx context.Context, tenantID, regionID string) ([]ShippingMethod, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT m.id, m.tenant_id, m.region_id, m.shipping_zone_id, m.name, m.price_cents, m.currency,
       m.min_order_cents, m.max_order_cents, m.channel_ids, m.postal_prefixes
FROM shipping_methods m
WHERE m.tenant_id = $1 AND m.region_id = $2
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShippingMethod{}
	for rows.Next() {
		m, err := scanMethod(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) GetMethodByID(ctx context.Context, tenantID, id string) (ShippingMethod, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, shipping_zone_id, name, price_cents, currency,
       min_order_cents, max_order_cents, channel_ids, postal_prefixes
FROM shipping_methods WHERE tenant_id = $1 AND id = $2
`, tenantID, id)
	return scanMethodRow(row)
}

func scanMethod(rows interface {
	Scan(dest ...any) error
}) (ShippingMethod, error) {
	var m ShippingMethod
	var min, max sql.NullInt64
	var ch, post []byte
	err := rows.Scan(&m.ID, &m.TenantID, &m.RegionID, &m.ShippingZoneID, &m.Name, &m.PriceCents, &m.Currency,
		&min, &max, &ch, &post)
	if err != nil {
		return ShippingMethod{}, err
	}
	if min.Valid {
		v := min.Int64
		m.MinOrderCents = &v
	}
	if max.Valid {
		v := max.Int64
		m.MaxOrderCents = &v
	}
	_ = json.Unmarshal(ch, &m.ChannelIDs)
	_ = json.Unmarshal(post, &m.PostalPrefixes)
	return m, nil
}

func scanMethodRow(row *sql.Row) (ShippingMethod, error) {
	var m ShippingMethod
	var min, max sql.NullInt64
	var ch, post []byte
	err := row.Scan(&m.ID, &m.TenantID, &m.RegionID, &m.ShippingZoneID, &m.Name, &m.PriceCents, &m.Currency,
		&min, &max, &ch, &post)
	if err != nil {
		return ShippingMethod{}, err
	}
	if min.Valid {
		v := min.Int64
		m.MinOrderCents = &v
	}
	if max.Valid {
		v := max.Int64
		m.MaxOrderCents = &v
	}
	_ = json.Unmarshal(ch, &m.ChannelIDs)
	_ = json.Unmarshal(post, &m.PostalPrefixes)
	return m, nil
}

// ZoneMatchesCountry returns true when the zone includes the ISO country code (case-insensitive).
func ZoneMatchesCountry(z ShippingZone, country string) bool {
	cc := strings.ToUpper(strings.TrimSpace(country))
	if cc == "" {
		return false
	}
	for _, c := range z.Countries {
		if strings.ToUpper(strings.TrimSpace(c)) == cc {
			return true
		}
	}
	return false
}

func MethodMatchesChannel(m ShippingMethod, channelID string) bool {
	ch := strings.TrimSpace(channelID)
	if len(m.ChannelIDs) == 0 {
		return true
	}
	for _, c := range m.ChannelIDs {
		if strings.TrimSpace(c) == ch {
			return true
		}
	}
	return false
}

func MethodMatchesPostal(m ShippingMethod, postal string) bool {
	p := strings.TrimSpace(strings.ToUpper(postal))
	if len(m.PostalPrefixes) == 0 {
		return true
	}
	if p == "" {
		return false
	}
	for _, pref := range m.PostalPrefixes {
		pref = strings.TrimSpace(strings.ToUpper(pref))
		if pref != "" && strings.HasPrefix(p, pref) {
			return true
		}
	}
	return false
}

func MethodMatchesOrderTotal(m ShippingMethod, total int64) bool {
	if m.MinOrderCents != nil && total < *m.MinOrderCents {
		return false
	}
	if m.MaxOrderCents != nil && total > *m.MaxOrderCents {
		return false
	}
	return true
}
