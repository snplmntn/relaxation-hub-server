-- Add void columns to ledger_entries table
ALTER TABLE ledger_entries
ADD COLUMN voided BOOLEAN DEFAULT FALSE,
ADD COLUMN voided_at TIMESTAMP,
ADD COLUMN voided_reason TEXT;

-- Index for filtering voided entries
CREATE INDEX idx_ledger_entries_voided ON ledger_entries(voided);
