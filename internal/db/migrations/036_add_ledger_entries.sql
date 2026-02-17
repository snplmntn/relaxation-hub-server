-- Migration: Add ledger_entries table for Double-Entry Accounting
-- This provides a unified financial journal for revenue, payouts, and expenses.

-- Create ENUM types for ledger entry classification
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_entry_type') THEN
        CREATE TYPE ledger_entry_type AS ENUM ('credit', 'debit');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_category') THEN
        CREATE TYPE ledger_category AS ENUM (
            'revenue',         -- Client payments (raw booking total)
            'commission',      -- Platform's cut (platform_fee)
            'payout',          -- Therapist earnings
            'expense',         -- Operating costs (rent, salaries, marketing)
            'refund',          -- Client refunds
            'adjustment'       -- Manual corrections
        );
    END IF;
END$$;

-- Create ledger_entries table
CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id       BIGSERIAL PRIMARY KEY,
    booking_id     BIGINT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    entry_type     ledger_entry_type NOT NULL,
    category       ledger_category NOT NULL,
    amount         NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    description    TEXT,
    entry_date     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by     BIGINT REFERENCES users(user_id) ON DELETE SET NULL  -- For manual entries (e.g., expenses)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_ledger_entries_booking_id ON ledger_entries(booking_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_entry_date ON ledger_entries(entry_date);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_category ON ledger_entries(category);

-- Backfill existing completed bookings into the ledger
-- This creates historical ledger entries for bookings that were completed before this migration.
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'commission'::ledger_category,
    COALESCE(b.platform_fee, 0),
    'Platform commission (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.platform_fee IS NOT NULL
  AND b.platform_fee > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'commission'
  );

-- Optional: Also backfill revenue (client payment) entries for full double-entry
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'revenue'::ledger_category,
    COALESCE(b.final_total, 0),
    'Client payment (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.final_total IS NOT NULL
  AND b.final_total > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'revenue'
  );

-- Optional: Also backfill payout (therapist earnings) entries
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'debit'::ledger_entry_type,
    'payout'::ledger_category,
    COALESCE(b.therapist_earnings, 0),
    'Therapist payout (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.therapist_earnings IS NOT NULL
  AND b.therapist_earnings > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'payout'
  );
