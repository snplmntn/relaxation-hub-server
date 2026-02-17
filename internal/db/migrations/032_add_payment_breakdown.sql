-- Migration: Add payment_breakdown JSONB column to bookings table
-- This stores itemized pricing: base_price, duration_markup, extensions_total, service_snapshot_name

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS payment_breakdown JSONB;

-- Add a comment for documentation
COMMENT ON COLUMN bookings.payment_breakdown IS 'Stores itemized price breakdown: base_price, duration_markup, extensions_total, service_snapshot_name';
