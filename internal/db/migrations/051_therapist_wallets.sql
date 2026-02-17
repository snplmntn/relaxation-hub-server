-- ============================================================================
-- Migration 041: Therapist Wallet System
-- ============================================================================
-- Provides real-time balance tracking for therapist earnings, payouts, and cash advances.
-- Integrates with the existing ledger system for full audit trail.

-- =============================================================================
-- 1. THERAPIST WALLETS (Balance Tracking)
-- =============================================================================
CREATE TABLE IF NOT EXISTS therapist_wallets (
    wallet_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE RESTRICT,
    
    -- Balance tracking
    available_balance NUMERIC(12,2) NOT NULL DEFAULT 0,  -- Ready for withdrawal
    pending_balance NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Held for 24h after booking
    
    -- Lifetime totals (for dashboard/reporting)
    total_earned NUMERIC(12,2) NOT NULL DEFAULT 0,       -- Sum of all earnings
    total_withdrawn NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Sum of all payouts
    total_advances NUMERIC(12,2) NOT NULL DEFAULT 0,     -- Sum of all cash advances
    
    -- Payout settings
    minimum_payout NUMERIC(12,2) NOT NULL DEFAULT 500,   -- Minimum withdrawal amount
    last_payout_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Balance can go negative due to cash advances
    CHECK (pending_balance >= 0)
);

-- Index for fast therapist lookup
CREATE INDEX IF NOT EXISTS idx_therapist_wallets_therapist ON therapist_wallets(therapist_id);

-- =============================================================================
-- 2. WALLET TRANSACTIONS (Audit Trail)
-- =============================================================================
-- Every balance change creates a transaction record for full traceability.
CREATE TABLE IF NOT EXISTS wallet_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    ledger_entry_id BIGINT REFERENCES ledger_entries(entry_id) ON DELETE SET NULL,
    
    -- Transaction details
    type VARCHAR(30) NOT NULL CHECK (type IN (
        'earning',           -- From completed booking
        'earning_released',  -- Moved from pending to available
        'payout',            -- Withdrawal to external account
        'cash_advance',      -- Pre-payment to therapist
        'advance_repayment', -- Deducted from earnings to repay advance
        'adjustment',        -- Manual correction by admin
        'refund_clawback'    -- Returned due to client refund
    )),
    
    amount NUMERIC(12,2) NOT NULL,              -- Positive for credit, negative for debit
    balance_after NUMERIC(12,2) NOT NULL,       -- Snapshot of available_balance after txn
    pending_after NUMERIC(12,2) NOT NULL DEFAULT 0, -- Snapshot of pending_balance after txn
    
    description TEXT,
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL, -- Admin who processed (if applicable)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_wallet_txns_wallet ON wallet_transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_booking ON wallet_transactions(booking_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_type ON wallet_transactions(type);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_created ON wallet_transactions(created_at DESC);

-- =============================================================================
-- 3. PAYOUT REQUESTS (Withdrawal Queue)
-- =============================================================================
-- Tracks therapist requests to withdraw funds.
CREATE TABLE IF NOT EXISTS payout_requests (
    request_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    amount NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    payout_method VARCHAR(30) NOT NULL CHECK (payout_method IN ('gcash', 'bank_transfer', 'cash')),
    account_details JSONB, -- {account_name, account_number, bank_name} for bank; {phone} for gcash
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending',     -- Awaiting admin approval
        'approved',    -- Approved, processing payment
        'completed',   -- Payment sent
        'rejected',    -- Rejected by admin
        'cancelled'    -- Cancelled by therapist
    )),
    
    -- Processing info
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    transaction_reference TEXT, -- External payment reference (bank txn, GCash ref)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payout_requests_wallet ON payout_requests(wallet_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_therapist ON payout_requests(therapist_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_status ON payout_requests(status);

-- =============================================================================
-- 4. CASH ADVANCE RECORDS
-- =============================================================================
-- Tracks cash advances given to therapists (to be repaid from future earnings).
CREATE TABLE IF NOT EXISTS cash_advances (
    advance_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    original_amount NUMERIC(12,2) NOT NULL CHECK (original_amount > 0),
    remaining_balance NUMERIC(12,2) NOT NULL CHECK (remaining_balance >= 0),
    repayment_rate NUMERIC(5,2) NOT NULL DEFAULT 50.00, -- % of each earning to deduct (e.g., 50%)
    
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN (
        'active',    -- Being repaid from earnings
        'paid_off',  -- Fully repaid
        'written_off' -- Admin wrote off the balance
    )),
    
    -- Approval info
    approved_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    reason TEXT,
    
    paid_off_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cash_advances_wallet ON cash_advances(wallet_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_therapist ON cash_advances(therapist_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_status ON cash_advances(status) WHERE status = 'active';

-- =============================================================================
-- 5. AUTO-CREATE WALLET ON THERAPIST PROFILE CREATION
-- =============================================================================
CREATE OR REPLACE FUNCTION create_therapist_wallet()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO therapist_wallets (therapist_id)
    VALUES (NEW.therapist_id)
    ON CONFLICT (therapist_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_create_wallet_on_therapist
    AFTER INSERT ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION create_therapist_wallet();

-- =============================================================================
-- 6. BACKFILL: Create wallets for existing therapists
-- =============================================================================
INSERT INTO therapist_wallets (therapist_id)
SELECT therapist_id FROM therapist_profiles
WHERE therapist_id NOT IN (SELECT therapist_id FROM therapist_wallets)
ON CONFLICT (therapist_id) DO NOTHING;

-- Backfill lifetime totals from ledger
-- Note: Only includes approved entries if status column exists (migration 030)
UPDATE therapist_wallets w
SET total_earned = COALESCE((
    SELECT SUM(amount) FROM ledger_entries le 
    WHERE le.category = 'payout' 
    AND le.entry_type = 'debit'
    AND (
        -- If status column exists, only count approved entries
        -- Otherwise, count all entries (for backwards compatibility)
        NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'ledger_entries' AND column_name = 'status'
        )
        OR le.status = 'approved'::ledger_entry_status
    )
    AND le.booking_id IN (SELECT booking_id FROM bookings WHERE therapist_id = w.therapist_id)
), 0);

-- Set available_balance to total_earned (since no withdrawals recorded yet in new system)
UPDATE therapist_wallets
SET available_balance = total_earned
WHERE total_earned > 0;
