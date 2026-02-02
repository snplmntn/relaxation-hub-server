-- Migration: 037_dynamic_travel_buffer.sql
-- Purpose: Add SQL functions for dynamic travel buffer calculation based on Haversine distance.

-- =============================================================================
-- 1. FUNCTION: calculate_distance_km
-- =============================================================================
-- Calculates the Great Circle distance between two points in kilometers.
CREATE OR REPLACE FUNCTION calculate_distance_km(lat1 float, lon1 float, lat2 float, lon2 float)
RETURNS float AS $$
DECLARE
    R float := 6371; -- Earth radius in km
    dLat float;
    dLon float;
    a float;
    c float;
BEGIN
    IF lat1 IS NULL OR lon1 IS NULL OR lat2 IS NULL OR lon2 IS NULL THEN
        RETURN NULL;
    END IF;

    dLat := radians(lat2 - lat1);
    dLon := radians(lon2 - lon1);
    
    -- Convert latitudes to radians for the formula
    a := sin(dLat/2) * sin(dLat/2) +
         sin(dLon/2) * sin(dLon/2) * cos(radians(lat1)) * cos(radians(lat2));
    c := 2 * atan2(sqrt(a), sqrt(1-a));
    RETURN R * c;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- =============================================================================
-- 2. FUNCTION: calculate_travel_buffer_minutes
-- =============================================================================
-- Returns the required buffer in minutes based on distance.
-- Logic:
--   < 0.5km: 0 minutes (Walking distance / Same building)
--   >= 0.5km: (Distance / Speed) + Setup Time
-- Assumptions:
--   - Average Speed: 20 km/h (Manila Traffic)
--   - Setup/Parking Time: 15 minutes
CREATE OR REPLACE FUNCTION calculate_travel_buffer_minutes(distance_km float)
RETURNS int AS $$
BEGIN
    -- If distance is unknown, assume they are far apart in different zones (Safe Default)
    IF distance_km IS NULL THEN
        RETURN 30; 
    END IF;

    IF distance_km < 0.5 THEN
        RETURN 0;
    END IF;

    -- Formula: (Dist / 20km/h * 60 mins) + 15 mins setup
    RETURN CEIL((distance_km / 20.0 * 60) + 15)::int;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
