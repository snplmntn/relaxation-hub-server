-- Daily accounting sheet.
--
-- The daily sales page already stores one remittance row per (business_date,
-- branch_id) with a single denomination count for the cash handed over. The
-- accounting sheet the owner actually reconciles against needs three more
-- things that the single scalar columns cannot express:
--
--   1. Closing staff count the cash they physically take home separately from
--      the cash that was remitted, so a second denomination breakdown
--      (closing_bill_*) is required alongside the existing bill_* counts.
--   2. GCash/Maya wallets are reconciled by balance-on-hand at closing, not by
--      the sales figures, so those balances are recorded explicitly.
--   3. The vault cash sits at the branch until the owner physically collects
--      it; vault_claimed/_at/_by is the audit trail for that hand-off.
--
-- Expenses and tips were previously squeezed into the others_deducted /
-- others_added / tips_total scalars, which meant whichever page saved last
-- clobbered the other's numbers and nobody could see what the money was for.
-- accounting_expenses and accounting_tips hold the line items; the daily sales
-- report derives those three scalars from them on read so must_be_zero stays
-- correct no matter which page wrote last.
--
-- Every statement is independently re-runnable; there is no down migration.

-- ============================================================================
-- A. Extend daily_sales_remittances
-- ============================================================================

ALTER TABLE daily_sales_remittances
    -- Wallet balances actually on hand at closing (reconciled, not derived).
    ADD COLUMN IF NOT EXISTS gcash_on_hand NUMERIC(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS maya_on_hand NUMERIC(10,2) NOT NULL DEFAULT 0,
    -- Owner ticks vault_claimed once they have physically taken the vault cash.
    ADD COLUMN IF NOT EXISTS vault_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS vault_claimed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS vault_claimed_by INT REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE daily_sales_remittances
    -- Second denomination count: cash the closing staff took, kept separate
    -- from the bill_* counts so both breakdowns survive a save from either page.
    ADD COLUMN IF NOT EXISTS closing_bill_1000 INT NOT NULL DEFAULT 0 CHECK (closing_bill_1000 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_500 INT NOT NULL DEFAULT 0 CHECK (closing_bill_500 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_200 INT NOT NULL DEFAULT 0 CHECK (closing_bill_200 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_100 INT NOT NULL DEFAULT 0 CHECK (closing_bill_100 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_50 INT NOT NULL DEFAULT 0 CHECK (closing_bill_50 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_20 INT NOT NULL DEFAULT 0 CHECK (closing_bill_20 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_10 INT NOT NULL DEFAULT 0 CHECK (closing_bill_10 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_5 INT NOT NULL DEFAULT 0 CHECK (closing_bill_5 >= 0),
    ADD COLUMN IF NOT EXISTS closing_bill_1 INT NOT NULL DEFAULT 0 CHECK (closing_bill_1 >= 0);

CREATE INDEX IF NOT EXISTS idx_daily_sales_remittances_vault_claimed
    ON daily_sales_remittances (vault_claimed, business_date);

-- ============================================================================
-- B. accounting_expenses - per-day, per-branch line-item expenses
-- ============================================================================

-- amount is SIGNED on purpose and deliberately has no >= 0 constraint: a
-- negative amount means cash came IN from outside the day's sales (owner funds
-- dropped at the branch, a vault transfer back in, a refunded expense). The
-- daily sales report splits the signed sum into others_deducted (positive) and
-- others_added (absolute value of the negatives).
CREATE TABLE IF NOT EXISTS accounting_expenses (
    expense_id    SERIAL PRIMARY KEY,
    business_date DATE NOT NULL,
    branch_id     INT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    label         TEXT NOT NULL,
    category      VARCHAR(32) NOT NULL CHECK (category IN (
                      'salary', 'reliever', 'transport', 'utilities', 'supplies',
                      'maintenance', 'vault_transfer', 'owner_funds', 'other'
                  )),
    amount        NUMERIC(12,2) NOT NULL, -- SIGNED: negative = cash came IN from outside sales
    created_by    INT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounting_expenses_date_branch
    ON accounting_expenses (business_date, branch_id);

-- ============================================================================
-- C. accounting_tips - per-day, per-branch, per-therapist tips
-- ============================================================================

CREATE TABLE IF NOT EXISTS accounting_tips (
    tip_id        SERIAL PRIMARY KEY,
    business_date DATE NOT NULL,
    branch_id     INT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    therapist_id  INT NOT NULL REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    amount        NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
    note          TEXT,
    created_by    INT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounting_tips_date_branch
    ON accounting_tips (business_date, branch_id);
