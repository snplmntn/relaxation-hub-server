-- Tracks cash physically collected (remitted) from therapists for cash-payment
-- bookings. A therapist's current cash on hand is derived as the sum of
-- completed cash bookings' final_total minus the sum of these remittances.
CREATE TABLE IF NOT EXISTS cash_remittances (
    remittance_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL REFERENCES users(user_id),
    amount NUMERIC(10, 2) NOT NULL CHECK (amount > 0),
    notes TEXT,
    remitted_by INT REFERENCES users(user_id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cash_remittances_therapist ON cash_remittances (therapist_id);
CREATE INDEX IF NOT EXISTS idx_cash_remittances_created_at ON cash_remittances (created_at);
