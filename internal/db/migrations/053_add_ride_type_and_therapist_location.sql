-- Migration 043: Add ride_type and therapist location support for logistics integration
-- This migration supports the Event-Driven Rider Integration feature

-- Add ride_type to rides table to distinguish outbound vs return trips
ALTER TABLE rides 
ADD COLUMN IF NOT EXISTS ride_type VARCHAR(20) DEFAULT 'outbound'
CHECK (ride_type IN ('outbound', 'return'));

-- Create index for ride-booking linkage queries
CREATE INDEX IF NOT EXISTS idx_rides_booking_id ON rides(booking_id) WHERE booking_id IS NOT NULL;

-- Create composite index for querying rides by type and status
CREATE INDEX IF NOT EXISTS idx_rides_type_status ON rides(ride_type, status);

-- Add home_address_id for therapists (future enhancement, optional for now)
-- Therapists can have a default pickup location separate from their branch
ALTER TABLE therapist_profiles
ADD COLUMN IF NOT EXISTS home_address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL;

-- Add index for therapist location resolution
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_home_address ON therapist_profiles(home_address_id) 
WHERE home_address_id IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN rides.ride_type IS 'Type of ride: outbound (therapist to client) or return (client back to therapist home/branch)';
COMMENT ON COLUMN therapist_profiles.home_address_id IS 'Therapist home address for ride pickups (optional, defaults to branch_id location if null)';
COMMENT ON INDEX idx_rides_booking_id IS 'Efficiently query rides associated with a specific booking';
COMMENT ON INDEX idx_rides_type_status IS 'Optimize queries filtering by ride type and status (e.g., pending return rides)';
