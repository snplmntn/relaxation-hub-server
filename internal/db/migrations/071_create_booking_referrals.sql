CREATE TABLE IF NOT EXISTS booking_referrals (
    booking_id BIGINT PRIMARY KEY REFERENCES bookings(booking_id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    other_notes TEXT,
    created_by_user_id BIGINT NOT NULL REFERENCES users(user_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_booking_referrals_created_at ON booking_referrals(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_booking_referrals_source ON booking_referrals(source);
