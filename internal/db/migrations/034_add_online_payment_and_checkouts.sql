-- Online payment via PayMongo.
--
-- 'online' is a new payment_method distinct from the existing manual 'gcash'
-- and 'bdo', which keep their meaning (customer transfers by hand and uploads a
-- receipt for staff to verify). An 'online' booking is paid through PayMongo
-- before it exists, so it never enters the receipt-verification queue.

ALTER TABLE booking_groups
    DROP CONSTRAINT IF EXISTS booking_groups_payment_method_check,
    ADD CONSTRAINT booking_groups_payment_method_check
        CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'maya', 'online'));

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS bookings_payment_method_check,
    ADD CONSTRAINT bookings_payment_method_check
        CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer', 'maya', 'online'));

-- A booking the customer has started paying for but which does not exist yet.
-- The full create request is parked here until PayMongo confirms payment; the
-- webhook then replays it through the normal booking-creation path.
CREATE TABLE IF NOT EXISTS booking_checkouts (
    checkout_id      BIGSERIAL PRIMARY KEY,
    reference        TEXT NOT NULL UNIQUE,
    client_id        INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,

    kind             TEXT NOT NULL CHECK (kind IN ('single', 'group')),
    channel          TEXT NOT NULL,
    request_payload  JSONB NOT NULL,
    amount           NUMERIC(10,2) NOT NULL CHECK (amount > 0),

    provider           TEXT NOT NULL DEFAULT 'paymongo',
    provider_session_id TEXT,
    checkout_url       TEXT,

    status           TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'failed', 'expired')),

    -- Set once fulfilled, so a replayed webhook is a no-op rather than a
    -- duplicate booking.
    event_id         TEXT UNIQUE,
    booking_id       INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    group_id         INT REFERENCES booking_groups(group_id) ON DELETE SET NULL,
    fulfil_note      TEXT,

    expires_at       TIMESTAMPTZ NOT NULL,
    paid_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_booking_checkouts_session
    ON booking_checkouts(provider_session_id);
CREATE INDEX IF NOT EXISTS idx_booking_checkouts_client
    ON booking_checkouts(client_id, created_at DESC);
