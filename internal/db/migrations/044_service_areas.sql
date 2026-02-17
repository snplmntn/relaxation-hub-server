-- ============================================================================
-- Migration 036: Service Areas (Unified Location Governance)
-- ============================================================================
-- Single-table approach for managing coverage, safety restrictions, and demand tracking.
-- SOTA Architecture: Configuration + Audit Log pattern.

-- =============================================================================
-- 1. SERVICE AREAS (The Configuration Catalog)
-- =============================================================================
-- Stores all cities and barangays with their operational status.
-- Uses PSGC codes for standardization with PhLocationService.

CREATE TABLE IF NOT EXISTS service_areas (
    area_id SERIAL PRIMARY KEY,
    psgc_code VARCHAR(20) NOT NULL UNIQUE,           -- PSGC standard code (city or barangay)
    parent_code VARCHAR(20),                          -- NULL for cities, city_code for barangays
    name VARCHAR(150) NOT NULL,                       -- Human-readable name
    level VARCHAR(20) NOT NULL CHECK (level IN ('region', 'province', 'city', 'barangay')),
    status VARCHAR(20) NOT NULL DEFAULT 'not_supported' 
        CHECK (status IN ('covered', 'banned', 'not_supported')),
    lat NUMERIC(9,6),                                 -- Centroid latitude for distance calc
    lng NUMERIC(9,6),                                 -- Centroid longitude for distance calc
    cached_request_count INT NOT NULL DEFAULT 0,      -- Denormalized count for fast dashboard queries
    min_booking_minutes INT NOT NULL DEFAULT 0,       -- Minimum booking duration for this area
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast status lookups during booking validation
CREATE INDEX IF NOT EXISTS idx_service_areas_psgc_code ON service_areas(psgc_code);
CREATE INDEX IF NOT EXISTS idx_service_areas_status ON service_areas(status);
CREATE INDEX IF NOT EXISTS idx_service_areas_parent ON service_areas(parent_code);

-- =============================================================================
-- 2. AREA COVERAGE REQUESTS (The Interest/Demand Log)
-- =============================================================================
-- Tracks individual user requests for coverage in unsupported areas.
-- Enables re-engagement campaigns when areas launch.

CREATE TABLE IF NOT EXISTS area_coverage_requests (
    request_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    psgc_code VARCHAR(20) NOT NULL,                   -- References service_areas, but area may not exist yet
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Prevent spam: one request per user per area
    CONSTRAINT uq_user_area_request UNIQUE (user_id, psgc_code)
);

-- Index for counting requests per area and for user lookups
CREATE INDEX IF NOT EXISTS idx_area_requests_psgc ON area_coverage_requests(psgc_code);
CREATE INDEX IF NOT EXISTS idx_area_requests_user ON area_coverage_requests(user_id);

-- =============================================================================
-- 3. TRIGGER: Auto-update cached_request_count
-- =============================================================================
-- Keeps the denormalized count in sync without manual updates.

CREATE OR REPLACE FUNCTION update_area_request_count()
RETURNS TRIGGER AS $$
BEGIN
    -- Update the count for the affected area (if it exists in service_areas)
    UPDATE service_areas 
    SET cached_request_count = (
        SELECT COUNT(*) FROM area_coverage_requests WHERE psgc_code = COALESCE(NEW.psgc_code, OLD.psgc_code)
    ),
    updated_at = NOW()
    WHERE psgc_code = COALESCE(NEW.psgc_code, OLD.psgc_code);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_area_request_count_update ON area_coverage_requests;

CREATE TRIGGER trg_area_request_count_update
    AFTER INSERT OR DELETE ON area_coverage_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_area_request_count();

-- =============================================================================
-- 4. SEED: Initial Launch Cities (NCR)
-- =============================================================================
-- Populate with initial covered areas. Coordinates are approximate centroids.

INSERT INTO service_areas (psgc_code, name, level, status, lat, lng) VALUES
    -- NCR Cities (Covered)
    ('137600000', 'Makati', 'city', 'covered', 14.5547, 121.0244),
    ('137500000', 'Taguig', 'city', 'covered', 14.5176, 121.0509),
    ('137400000', 'Pasig', 'city', 'covered', 14.5764, 121.0851)
ON CONFLICT (psgc_code) DO UPDATE SET 
    status = EXCLUDED.status,
    lat = EXCLUDED.lat,
    lng = EXCLUDED.lng,
    updated_at = NOW();

-- Example: Banned Barangay (for testing/demo)
-- INSERT INTO service_areas (psgc_code, parent_code, name, level, status) VALUES
--     ('137600001', '137600000', 'Barangay Test', 'barangay', 'banned');
