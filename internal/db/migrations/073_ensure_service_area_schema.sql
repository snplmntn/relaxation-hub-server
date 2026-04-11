-- Migration 073: Ensure service area schema exists.
-- This handles both new environments where tables are missing and old ones that need psgc_code -> area_key.

-- 1. Create service_areas table if missing
CREATE TABLE IF NOT EXISTS service_areas (
    area_id BIGSERIAL PRIMARY KEY,
    area_key TEXT NOT NULL UNIQUE,
    parent_code TEXT,
    name TEXT NOT NULL,
    level TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'not_supported',
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    cached_request_count INTEGER NOT NULL DEFAULT 0,
    min_booking_minutes INTEGER NOT NULL DEFAULT 60,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Create area_coverage_requests table if missing
CREATE TABLE IF NOT EXISTS area_coverage_requests (
    request_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id),
    area_key TEXT NOT NULL REFERENCES service_areas(area_key),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, area_key)
);

-- 3. Ensure area_key column name (redundant but safe after rename logic in 072)
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns 
    WHERE table_name = 'service_areas' AND column_name = 'psgc_code'
  ) THEN
    ALTER TABLE service_areas RENAME COLUMN psgc_code TO area_key;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns 
    WHERE table_name = 'area_coverage_requests' AND column_name = 'psgc_code'
  ) THEN
    ALTER TABLE area_coverage_requests RENAME COLUMN psgc_code TO area_key;
  END IF;
END$$;

-- 4. Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_service_areas_status ON service_areas(status);
CREATE INDEX IF NOT EXISTS idx_service_areas_level ON service_areas(level);
CREATE INDEX IF NOT EXISTS idx_area_coverage_requests_area_key ON area_coverage_requests(area_key);
