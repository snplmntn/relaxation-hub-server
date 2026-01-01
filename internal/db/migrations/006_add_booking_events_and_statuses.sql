-- ============================================================================
-- Migration: 006 - Booking events + assignment/arrival/cancellation timestamps
-- ============================================================================
-- Adds audit events for bookings and a few helpful timestamp/cancellation columns
-- This migration is idempotent (uses IF NOT EXISTS checks)

BEGIN;

-- Add new timestamp and cancellation columns to bookings
ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS therapist_arrived_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS no_show_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS cancelled_by VARCHAR(20),
    ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS cancellation_reason TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create booking_events table to record timeline and important actions
CREATE TABLE IF NOT EXISTS booking_events (
    event_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL, -- e.g. 'created','assigned','payment_succeeded','therapist_arrived',etc.
    actor_id INT REFERENCES users(user_id) ON DELETE SET NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for booking_events to support timeline queries
CREATE INDEX IF NOT EXISTS idx_booking_events_booking ON booking_events(booking_id);
CREATE INDEX IF NOT EXISTS idx_booking_events_type ON booking_events(event_type);
CREATE INDEX IF NOT EXISTS idx_booking_events_created_at ON booking_events(created_at DESC);

-- Add updated_at trigger for bookings if not already present
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_bookings_updated_at'
    ) THEN
        CREATE TRIGGER update_bookings_updated_at
            BEFORE UPDATE ON bookings
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMIT;

-- Helpful comments
COMMENT ON TABLE booking_events IS 'Timeline of booking-related events used for UI timelines, auditing and idempotency.';
