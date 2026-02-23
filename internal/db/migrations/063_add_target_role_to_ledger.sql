-- Migration 063: Add target_role to ledger_entries for unified payout tracking
-- Apply manually: psql -d <db> -f 063_add_target_role_to_ledger.sql

ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS target_role VARCHAR(20);

-- Backfill: existing entries with a target_user_id are therapist payouts/settlements
UPDATE ledger_entries
SET target_role = 'therapist'
WHERE target_user_id IS NOT NULL AND target_role IS NULL;

-- Composite indexes for role-aware balance queries
CREATE INDEX IF NOT EXISTS idx_ledger_target_role_user
    ON ledger_entries(target_role, target_user_id, category, voided);

CREATE INDEX IF NOT EXISTS idx_ledger_date_role
    ON ledger_entries(entry_date, target_role, category, voided);
