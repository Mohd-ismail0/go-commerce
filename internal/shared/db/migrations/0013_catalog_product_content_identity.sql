ALTER TABLE products
ADD COLUMN IF NOT EXISTS slug TEXT,
ADD COLUMN IF NOT EXISTS description TEXT,
ADD COLUMN IF NOT EXISTS seo_title TEXT,
ADD COLUMN IF NOT EXISTS seo_description TEXT,
ADD COLUMN IF NOT EXISTS metadata JSONB,
ADD COLUMN IF NOT EXISTS external_reference TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_products_tenant_region_slug
ON products (tenant_id, region_id, slug)
WHERE slug IS NOT NULL;
