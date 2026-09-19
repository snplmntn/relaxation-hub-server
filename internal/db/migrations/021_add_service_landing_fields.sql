-- Adds landing-page presentation fields to services so the public site can be
-- driven by a curated subset of the catalog managed from the Super Admin dashboard.
ALTER TABLE services ADD COLUMN IF NOT EXISTS subtitle TEXT;
ALTER TABLE services ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE services ADD COLUMN IF NOT EXISTS featured_order INT NOT NULL DEFAULT 0;
