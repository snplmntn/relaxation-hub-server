-- ============================================================================
-- RIDER PAYOUT METHODS
-- ============================================================================

CREATE TABLE IF NOT EXISTS rider_payout_methods (
    id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method_type VARCHAR(20) NOT NULL CHECK (method_type IN ('bank', 'gcash', 'paymaya', 'grabpay')),
    provider_name VARCHAR(100) NOT NULL, -- e.g., 'BDO', 'BPI', 'GCash'
    account_number VARCHAR(100) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE rider_payout_methods IS 'Stored payout destinations for riders';
COMMENT ON COLUMN rider_payout_methods.method_type IS 'Type of payout: bank account or e-wallet';

CREATE INDEX IF NOT EXISTS idx_rider_payout_methods_rider ON rider_payout_methods(rider_id);

-- Ensure only one default per rider
CREATE UNIQUE INDEX IF NOT EXISTS idx_rider_payout_methods_default 
ON rider_payout_methods(rider_id) 
WHERE is_default = TRUE;

-- ============================================================================
-- LINK TRANSACTIONS TO PAYOUT METHODS
-- ============================================================================

ALTER TABLE rider_transactions 
ADD COLUMN IF NOT EXISTS payout_method_id INT REFERENCES rider_payout_methods(id) ON DELETE SET NULL;

COMMENT ON COLUMN rider_transactions.payout_method_id IS 'Link to the specific payout destination used for this transaction';
