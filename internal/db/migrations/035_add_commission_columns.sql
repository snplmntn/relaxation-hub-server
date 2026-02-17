-- Migration: Add commission tracking columns
-- 1. Add therapist_commission to services table
-- 2. Add therapist_earnings and platform_fee to bookings table

-- Services: Fixed commission amount per service
ALTER TABLE services ADD COLUMN IF NOT EXISTS therapist_commission NUMERIC(10,2);

-- Comment for documentation
COMMENT ON COLUMN services.therapist_commission IS 'The fixed amount the therapist earns for the base duration of this service';

-- Bookings: Snapshot of the financial split at completion
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS therapist_earnings NUMERIC(10,2);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS platform_fee NUMERIC(10,2);

COMMENT ON COLUMN bookings.therapist_earnings IS 'Final amount payable to therapist (base + extensions share)';
COMMENT ON COLUMN bookings.platform_fee IS 'Amount retained by platform (final_total - therapist_earnings)';
