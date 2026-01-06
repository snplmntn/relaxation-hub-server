-- Migration: Add proof_url and status columns to ledger_entries
-- Purpose: Enable audit-ready expense tracking with optional approval workflow

-- Add ledger entry status enum
DO $$ BEGIN
    CREATE TYPE ledger_entry_status AS ENUM ('pending', 'approved', 'rejected');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Add new columns
ALTER TABLE ledger_entries
ADD COLUMN IF NOT EXISTS proof_url TEXT,
ADD COLUMN IF NOT EXISTS status ledger_entry_status NOT NULL DEFAULT 'pending',
ADD COLUMN IF NOT EXISTS reviewed_by BIGINT REFERENCES users(user_id),
ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Create index for filtering by status
CREATE INDEX IF NOT EXISTS idx_ledger_entries_status ON ledger_entries(status);

-- Comment for documentation
COMMENT ON COLUMN ledger_entries.proof_url IS 'URL to receipt/invoice image for expense substantiation';
COMMENT ON COLUMN ledger_entries.status IS 'pending=awaiting review, approved=in the books, rejected=denied';
COMMENT ON COLUMN ledger_entries.reviewed_by IS 'User who approved or rejected the entry';
COMMENT ON COLUMN ledger_entries.reviewed_at IS 'When the entry was reviewed (approved/rejected)';
