-- ride_offers: parallel to booking_offers, tracks individual ride offers to riders
CREATE TABLE IF NOT EXISTS ride_offers (
    offer_id BIGSERIAL PRIMARY KEY,
    ride_id BIGINT NOT NULL REFERENCES rides(ride_id) ON DELETE CASCADE,
    rider_id BIGINT NOT NULL REFERENCES rider_profiles(rider_id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, accepted, declined, expired
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    UNIQUE(ride_id, rider_id)
);

CREATE INDEX IF NOT EXISTS idx_ride_offers_ride_id ON ride_offers(ride_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_rider_id ON ride_offers(rider_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_status ON ride_offers(status);
CREATE INDEX IF NOT EXISTS idx_ride_offers_expires_at ON ride_offers(expires_at) WHERE status = 'pending';

-- Add scheduled_for to rides (nullable — set for return rides)
ALTER TABLE rides ADD COLUMN IF NOT EXISTS scheduled_for TIMESTAMPTZ;
