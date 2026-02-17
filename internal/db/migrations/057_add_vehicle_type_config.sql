-- Migration 047: Add vehicle_type to ride_pricing_config
-- Allows dynamic vehicle selection for dispatch calculations

ALTER TABLE ride_pricing_config
ADD COLUMN IF NOT EXISTS default_vehicle_type VARCHAR(50) DEFAULT 'motorcycle';

COMMENT ON COLUMN ride_pricing_config.default_vehicle_type IS 'Default vehicle profile for routing (motorcycle, car, bicycle, walking)';
