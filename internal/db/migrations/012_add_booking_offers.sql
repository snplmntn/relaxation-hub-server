CREATE TABLE booking_offers (
    offer_id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    therapist_id BIGINT NOT NULL REFERENCES users(user_id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, accepted, declined, expired
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE(booking_id, therapist_id)
);

CREATE INDEX idx_booking_offers_booking_id ON booking_offers(booking_id);
CREATE INDEX idx_booking_offers_therapist_id ON booking_offers(therapist_id);
CREATE INDEX idx_booking_offers_status ON booking_offers(status);
