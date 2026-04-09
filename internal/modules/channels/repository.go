package channels

import (
	"context"
	"database/sql"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SlugTaken(ctx context.Context, tenantID, regionID, slug, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM channels
  WHERE tenant_id = $1 AND region_id = $2 AND slug = $3
    AND ($4::text = '' OR id <> $4)
)
`, tenantID, regionID, slug, excludeID).Scan(&exists)
	return exists, err
}

func (r *Repository) Save(ctx context.Context, ch Channel) (Channel, error) {
	var out Channel
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
INSERT INTO channels (id, tenant_id, region_id, slug, name, default_currency, default_country, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  region_id = EXCLUDED.region_id,
  slug = EXCLUDED.slug,
  name = EXCLUDED.name,
  default_currency = EXCLUDED.default_currency,
  default_country = EXCLUDED.default_country,
  is_active = EXCLUDED.is_active,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, slug, name, default_currency, default_country, is_active, updated_at
`, ch.ID, ch.TenantID, ch.RegionID, ch.Slug, ch.Name, ch.DefaultCurrency, ch.DefaultCountry, ch.IsActive).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.Slug, &out.Name,
		&out.DefaultCurrency, &out.DefaultCountry, &out.IsActive, &updatedAt,
	)
	if err != nil {
		return Channel{}, err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return out, nil
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string) ([]Channel, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, slug, name, default_currency, default_country, is_active, updated_at
FROM channels
WHERE tenant_id = $1 AND region_id = $2
ORDER BY updated_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		var updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Slug, &c.Name,
			&c.DefaultCurrency, &c.DefaultCountry, &c.IsActive, &updatedAt); err != nil {
			return nil, err
		}
		c.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) ChannelExists(ctx context.Context, tenantID, regionID, channelID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM channels
  WHERE tenant_id = $1 AND region_id = $2 AND id = $3
)
`, tenantID, regionID, channelID).Scan(&ok)
	return ok, err
}

func (r *Repository) ProductExists(ctx context.Context, tenantID, regionID, productID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM products
  WHERE tenant_id = $1 AND region_id = $2 AND id = $3
)
`, tenantID, regionID, productID).Scan(&ok)
	return ok, err
}

func (r *Repository) GetProductListingByKeys(ctx context.Context, tenantID, regionID, channelID, productID string) (ProductChannelListing, bool, error) {
	var row ProductChannelListing
	var updatedAt time.Time
	var publishedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, product_id, channel_id, is_published, visible_in_listings, published_at, updated_at
FROM product_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3 AND product_id = $4
`, tenantID, regionID, channelID, productID).Scan(
		&row.ID, &row.TenantID, &row.RegionID, &row.ProductID, &row.ChannelID,
		&row.IsPublished, &row.VisibleInListings, &publishedAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return ProductChannelListing{}, false, nil
	}
	if err != nil {
		return ProductChannelListing{}, false, err
	}
	row.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if publishedAt.Valid {
		row.PublishedAt = publishedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return row, true, nil
}

func (r *Repository) ListProductListingsByChannel(ctx context.Context, tenantID, regionID, channelID string) ([]ProductChannelListing, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, product_id, channel_id, is_published, visible_in_listings, published_at, updated_at
FROM product_channel_listings
WHERE tenant_id = $1 AND region_id = $2 AND channel_id = $3
ORDER BY updated_at DESC
`, tenantID, regionID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProductChannelListing
	for rows.Next() {
		var row ProductChannelListing
		var updatedAt time.Time
		var publishedAt sql.NullTime
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.RegionID, &row.ProductID, &row.ChannelID,
			&row.IsPublished, &row.VisibleInListings, &publishedAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		row.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		if publishedAt.Valid {
			row.PublishedAt = publishedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) SaveProductListing(ctx context.Context, row ProductChannelListing, publishedAt sql.NullTime) (ProductChannelListing, error) {
	var out ProductChannelListing
	var updatedAt time.Time
	var publishedOut sql.NullTime
	err := r.db.QueryRowContext(ctx, `
INSERT INTO product_channel_listings (
  id, tenant_id, region_id, product_id, channel_id, is_published, visible_in_listings, published_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
ON CONFLICT (tenant_id, region_id, product_id, channel_id) DO UPDATE SET
  is_published = EXCLUDED.is_published,
  visible_in_listings = EXCLUDED.visible_in_listings,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
RETURNING id, tenant_id, region_id, product_id, channel_id, is_published, visible_in_listings, published_at, updated_at
`, row.ID, row.TenantID, row.RegionID, row.ProductID, row.ChannelID, row.IsPublished, row.VisibleInListings, publishedAt).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.ProductID, &out.ChannelID,
		&out.IsPublished, &out.VisibleInListings, &publishedOut, &updatedAt,
	)
	if err != nil {
		return ProductChannelListing{}, err
	}
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if publishedOut.Valid {
		out.PublishedAt = publishedOut.Time.UTC().Format(time.RFC3339Nano)
	}
	return out, nil
}
