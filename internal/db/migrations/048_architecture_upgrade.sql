-- Migration 040: Architecture Upgrade (Bundles, Durable State)

-- 1. Create Booking Offer Items Table (For Bundles)
CREATE TABLE IF NOT EXISTS booking_offer_items (
    offer_id BIGINT NOT NULL REFERENCES booking_offers(offer_id) ON DELETE CASCADE,
    booking_id BIGINT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    estimated_earnings NUMERIC(10, 2) DEFAULT 0,
    PRIMARY KEY (offer_id, booking_id)
);

-- Add columns to booking_offers if they don't exist
ALTER TABLE booking_offers
ADD COLUMN IF NOT EXISTS estimated_earnings NUMERIC(10, 2),
ADD COLUMN IF NOT EXISTS is_bundle BOOLEAN DEFAULT FALSE;

-- 2. Relax booking_offers.booking_id constraint
-- We keep the column for backward compatibility (and potentially as a "representative" ID),
-- but new logic might rely purely on the items table for multi-booking offers.
ALTER TABLE booking_offers ALTER COLUMN booking_id DROP NOT NULL;

-- 3. Add Workflow State to Assignment Queue (Durable Execution)
ALTER TABLE booking_assignment_queue
ADD COLUMN IF NOT EXISTS workflow_state VARCHAR(50) DEFAULT 'init',
ADD COLUMN IF NOT EXISTS workflow_data JSONB DEFAULT '{}';

-- 4. Index for State Lookups?
-- (Optional: For now, the queue is small enough, fast key lookup by booking_id is primary)
