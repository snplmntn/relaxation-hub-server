-- Add notes column to payments table
ALTER TABLE payments ADD COLUMN IF NOT EXISTS notes TEXT;

-- Add notes column to ledger_entries table
ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS notes TEXT;

-- Update payments status check constraint to include 'rejected'
-- First drop the old constraint, then add new one
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'expired', 'rejected'));
