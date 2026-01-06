-- Migration: Add target_user_id for wallet tracking and settlement category
-- Purpose: Track owed balances per user (therapist) and record payouts.

-- 1. Add target_user_id column
ALTER TABLE ledger_entries
ADD COLUMN IF NOT EXISTS target_user_id BIGINT REFERENCES users(user_id);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_target_user ON ledger_entries(target_user_id);

-- 2. Add 'settlement' to ledger_category ENUM
-- Note: 'IF NOT EXISTS' for ENUM values requires PostgreSQL 12+.
-- If earlier version, we catch error. But assuming 12+ for this stack.
ALTER TYPE ledger_category ADD VALUE IF NOT EXISTS 'settlement';

-- 3. Backfill target_user_id for existing Payout entries from Bookings
UPDATE ledger_entries le
SET target_user_id = b.therapist_id
FROM bookings b
WHERE le.booking_id = b.booking_id
  AND le.category = 'payout'
  AND le.target_user_id IS NULL;

-- 4. Comment
COMMENT ON COLUMN ledger_entries.target_user_id IS 'The user who owns this balance (e.g., therapist for payouts).';
