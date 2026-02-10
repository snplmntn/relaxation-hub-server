-- ============================================================================
-- RIDER WALLET SYSTEM
-- ============================================================================

-- Rider Wallets (mirrors therapist_wallets design)
CREATE TABLE IF NOT EXISTS rider_wallets (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    balance_cents INT NOT NULL DEFAULT 0,
    total_earned_cents INT NOT NULL DEFAULT 0,
    total_withdrawn_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_walletbalance_non_negative CHECK (balance_cents >= 0),
    CONSTRAINT rider_wallet_totals_consistent CHECK (total_earned_cents >= total_withdrawn_cents)
);

COMMENT ON TABLE rider_wallets IS 'Tracks rider earnings and payout balances';
COMMENT ON COLUMN rider_wallets.balance_cents IS 'Current available balance in cents';
COMMENT ON COLUMN rider_wallets.total_earned_cents IS 'Lifetime earnings from all rides';
COMMENT ON COLUMN rider_wallets.total_withdrawn_cents IS 'Total amount withdrawn via payouts';

-- Rider Transactions (similar to therapist ledger_entries)
CREATE TABLE IF NOT EXISTS rider_transactions (
    transaction_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('ride_earning', 'payout', 'adjustment', 'bonus')),
    amount_cents INT NOT NULL,
    ride_id INT REFERENCES rides(ride_id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    
    CONSTRAINT rider_transaction_amount_non_zero CHECK (amount_cents != 0)
);

COMMENT ON TABLE rider_transactions IS 'Transaction history for rider wallet operations';
COMMENT ON COLUMN rider_transactions.transaction_type IS 'Type of transaction: ride_earning (credit), payout (debit), adjustment (admin), bonus (credit)';
COMMENT ON COLUMN rider_transactions.amount_cents IS 'Transaction amount in cents (positive for credit, negative for debit)';
COMMENT ON COLUMN rider_transactions.status IS 'Transaction status for async operations like payouts';

CREATE INDEX IF NOT EXISTS idx_rider_transactions_rider ON rider_transactions(rider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_ride ON rider_transactions(ride_id);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_status ON rider_transactions(status) WHERE status = 'pending';

-- ============================================================================
-- RIDER PERFORMANCE METRICS
-- ============================================================================

CREATE TABLE IF NOT EXISTS rider_performance_metrics (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    total_offers_received INT NOT NULL DEFAULT 0,
    total_rides_accepted INT NOT NULL DEFAULT 0,
    total_rides_completed INT NOT NULL DEFAULT 0,
    total_rides_cancelled INT NOT NULL DEFAULT 0,
    acceptance_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    completion_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    average_rating DECIMAL(3,2) DEFAULT NULL,
    total_ratings INT NOT NULL DEFAULT 0,
    rating_sum INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_acceptance_rate_valid CHECK (acceptance_rate >= 0 AND acceptance_rate <= 100),
    CONSTRAINT rider_completion_rate_valid CHECK (completion_rate >= 0 AND completion_rate <= 100),
    CONSTRAINT rider_average_rating_valid CHECK (average_rating IS NULL OR (average_rating >= 1 AND average_rating <= 5))
);

COMMENT ON TABLE rider_performance_metrics IS 'Tracks rider performance and ratings for quality control';
COMMENT ON COLUMN rider_performance_metrics.acceptance_rate IS 'Percentage of offers accepted (rides_accepted / offers_received * 100)';
COMMENT ON COLUMN rider_performance_metrics.completion_rate IS 'Percentage of rides completed (rides_completed / rides_accepted * 100)';
COMMENT ON COLUMN rider_performance_metrics.average_rating IS 'Average passenger rating (1-5 stars)';

-- ============================================================================
-- SAFETY FEATURES
-- ============================================================================

-- Emergency Contacts for Riders
CREATE TABLE IF NOT EXISTS rider_emergency_contacts (
    contact_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    full_name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    relationship VARCHAR(50),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_emergency_phone_format CHECK (phone_number ~ '^\+?[0-9]{10,15}$')
);

COMMENT ON TABLE rider_emergency_contacts IS 'Emergency contacts for rider safety features (SOS, trip sharing)';
COMMENT ON COLUMN rider_emergency_contacts.is_primary IS 'Primary contact receives alerts first';

CREATE INDEX IF NOT EXISTS idx_rider_emergency_contacts_rider ON rider_emergency_contacts(rider_id);

-- ============================================================================
-- EXTEND RIDES TABLE
-- ============================================================================

-- Add rider earnings column to rides table
ALTER TABLE rides ADD COLUMN IF NOT EXISTS rider_earnings_cents INT;

COMMENT ON COLUMN rides.rider_earnings_cents IS 'Amount credited to rider wallet upon ride completion (calculated from fare)';

-- ============================================================================
-- TRIGGERS FOR AUTO-CALCULATION
-- ============================================================================

-- Trigger to update rider wallet when ride earnings are recorded
CREATE OR REPLACE FUNCTION update_rider_wallet_on_earning()
RETURNS TRIGGER AS $$
BEGIN
    -- Only process if rider_earnings_cents is set and status is completed
    IF NEW.rider_earnings_cents IS NOT NULL AND NEW.status = 'completed' AND 
       (OLD.status IS NULL OR OLD.status != 'completed') THEN
        
        -- Ensure wallet exists
        INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents)
        VALUES (NEW.rider_id, 0, 0)
        ON CONFLICT (rider_id) DO NOTHING;
        
        -- Update wallet
        UPDATE rider_wallets
        SET 
            balance_cents = balance_cents + NEW.rider_earnings_cents,
            total_earned_cents = total_earned_cents + NEW.rider_earnings_cents,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
        -- Create transaction record
        INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, ride_id, status, description)
        VALUES (
            NEW.rider_id,
            'ride_earning',
            NEW.rider_earnings_cents,
            NEW.ride_id,
            'completed',
            FORMAT('Earnings from ride #%s', NEW.ride_id)
        );
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_rider_wallet ON rides;
CREATE TRIGGER trigger_update_rider_wallet
AFTER UPDATE ON rides
FOR EACH ROW
EXECUTE FUNCTION update_rider_wallet_on_earning();

-- Trigger to update performance metrics when rider accepts/completes rides
CREATE OR REPLACE FUNCTION update_rider_performance_metrics()
RETURNS TRIGGER AS $$
DECLARE
    v_acceptance_rate DECIMAL(5,2);
    v_completion_rate DECIMAL(5,2);
BEGIN
    -- Ensure metrics row exists
    INSERT INTO rider_performance_metrics (rider_id)
    VALUES (NEW.rider_id)
    ON CONFLICT (rider_id) DO NOTHING;
    
    -- Update based on status change
    IF NEW.status = 'accepted' AND (OLD.status IS NULL OR OLD.status = 'pending') THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_accepted = total_rides_accepted + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
    ELSIF NEW.status = 'completed' AND OLD.status != 'completed' THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_completed = total_rides_completed + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
    ELSIF NEW.status = 'cancelled' AND OLD.status NOT IN ('cancelled', 'completed') THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_cancelled = total_rides_cancelled + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
    END IF;
    
    -- Recalculate rates
    UPDATE rider_performance_metrics
    SET 
        acceptance_rate = CASE 
            WHEN total_offers_received > 0 
            THEN (total_rides_accepted::DECIMAL / total_offers_received * 100)
            ELSE 0 
        END,
        completion_rate = CASE 
            WHEN total_rides_accepted > 0 
            THEN (total_rides_completed::DECIMAL / total_rides_accepted * 100)
            ELSE 0 
        END
    WHERE rider_id = NEW.rider_id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_rider_performance ON rides;
CREATE TRIGGER trigger_update_rider_performance
AFTER UPDATE ON rides
FOR EACH ROW
WHEN (NEW.rider_id IS NOT NULL)
EXECUTE FUNCTION update_rider_performance_metrics();

-- ============================================================================
-- INITIAL DATA / SEED
-- ============================================================================

-- Create wallet and metrics for existing riders (retroactive)
INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents)
SELECT DISTINCT u.user_id, 0, 0
FROM users u
WHERE u.role = 'rider'
ON CONFLICT (rider_id) DO NOTHING;

INSERT INTO rider_performance_metrics (rider_id)
SELECT DISTINCT u.user_id
FROM users u
WHERE u.role = 'rider'
ON CONFLICT (rider_id) DO NOTHING;
