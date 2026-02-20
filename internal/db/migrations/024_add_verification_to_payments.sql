-- Migration: Add verified_by column to payments table for tracking who verified the payment
-- This complements the existing proof_url added in 023

ALTER TABLE payments ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS verified_by INT REFERENCES users(user_id) ON DELETE SET NULL;

-- Index for finding verified payments
CREATE INDEX IF NOT EXISTS idx_payments_verified ON payments(verified_at) WHERE verified_at IS NOT NULL;
