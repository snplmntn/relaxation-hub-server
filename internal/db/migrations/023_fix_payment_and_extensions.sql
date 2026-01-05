-- Migration: Payment proof tracking and extension wait time
-- 1. Add proof_url to payments table for tracking uploaded payment proofs
-- 2. Add extension_wait_seconds to bookings for accurate time tracking during approval waits

-- Add proof_url column to payments table
ALTER TABLE payments ADD COLUMN IF NOT EXISTS proof_url TEXT;

-- Add extension_wait_seconds to bookings table
-- This tracks time spent waiting for extension approval (separate from pause time)
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS extension_wait_seconds INT DEFAULT 0;

-- Add 'cancelled' status to extension_requests if not already present
ALTER TABLE booking_extension_requests DROP CONSTRAINT IF EXISTS booking_extension_requests_status_check;
ALTER TABLE booking_extension_requests ADD CONSTRAINT booking_extension_requests_status_check 
    CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled'));

-- Index for payment proof lookups
CREATE INDEX IF NOT EXISTS idx_payments_proof_url ON payments(proof_url) WHERE proof_url IS NOT NULL;
