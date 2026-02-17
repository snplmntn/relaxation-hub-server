-- Enable PostGIS for geospatial operations
CREATE EXTENSION IF NOT EXISTS postgis;

-- Rider Profiles
CREATE TABLE IF NOT EXISTS rider_profiles (
    rider_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    vehicle_type VARCHAR(50), -- 'motorcycle', 'car', 'suv'
    license_plate VARCHAR(20),
    license_number VARCHAR(50),
    is_online BOOLEAN DEFAULT FALSE,
    current_location GEOGRAPHY(POINT, 4326), -- SOTA: geography type for accurate distances
    last_location_update TIMESTAMPTZ,
    rating DECIMAL(3,2) DEFAULT 5.0,
    total_trips INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- GiST Index for fast geospatial lookups (SOTA 2026)
CREATE INDEX IF NOT EXISTS idx_rider_location ON rider_profiles USING GIST(current_location);
CREATE INDEX IF NOT EXISTS idx_rider_online ON rider_profiles(is_online) WHERE is_online = true;

-- Rides Table
CREATE TABLE IF NOT EXISTS rides (
    ride_id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT REFERENCES rider_profiles(rider_id),
    passenger_id BIGINT NOT NULL REFERENCES users(user_id), -- Therapist
    booking_id BIGINT REFERENCES bookings(booking_id), -- Optional link to massage booking
    
    -- Pickup (Client location)
    pickup_lat DECIMAL(10, 7) NOT NULL,
    pickup_long DECIMAL(10, 7) NOT NULL,
    pickup_address TEXT,
    
    -- Dropoff (Homebase or home)
    dropoff_lat DECIMAL(10, 7) NOT NULL,
    dropoff_long DECIMAL(10, 7) NOT NULL,
    dropoff_address TEXT,
    
    -- Pricing (Snapshot for historical accuracy)
    distance_km DECIMAL(6,2),
    pricing_snapshot JSONB, -- {base_rate, per_km_rate, surge_multiplier, final_fare}
    
    -- Status Flow
    status VARCHAR(30) DEFAULT 'pending', 
    -- 'pending' -> 'offered' -> 'accepted' -> 'arrived_pickup' -> 'in_progress' -> 'completed' / 'cancelled'
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    offered_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    arrived_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancellation_reason TEXT,
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rides_rider ON rides(rider_id);
CREATE INDEX IF NOT EXISTS idx_rides_passenger ON rides(passenger_id);
CREATE INDEX IF NOT EXISTS idx_rides_status ON rides(status);

-- Ride Pricing Configuration (Admin-Configurable)
CREATE TABLE IF NOT EXISTS ride_pricing_config (
    config_id SERIAL PRIMARY KEY,
    config_key VARCHAR(50) UNIQUE DEFAULT 'default',
    base_distance_km DECIMAL(4,2) DEFAULT 3.0,
    base_rate DECIMAL(8,2) DEFAULT 50.0,
    per_km_rate DECIMAL(8,2) DEFAULT 10.0,
    per_100m_rate DECIMAL(8,2) DEFAULT 1.0, -- Granular pricing (.1 km)
    min_fare DECIMAL(8,2) DEFAULT 50.0,
    max_fare DECIMAL(8,2) DEFAULT 150.0,
    surge_enabled BOOLEAN DEFAULT FALSE,
    surge_multiplier DECIMAL(3,2) DEFAULT 1.0, -- SOTA: Dynamic pricing support
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO ride_pricing_config (config_key) VALUES ('default') ON CONFLICT DO NOTHING;
