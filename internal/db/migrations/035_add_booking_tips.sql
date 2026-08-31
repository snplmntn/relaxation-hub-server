ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS tip_amount NUMERIC(10, 2) NOT NULL DEFAULT 0
    CHECK (tip_amount >= 0 AND tip_amount <= 10000);

ALTER TABLE booking_groups
    ADD COLUMN IF NOT EXISTS tip_amount NUMERIC(10, 2) NOT NULL DEFAULT 0
    CHECK (tip_amount >= 0 AND tip_amount <= 10000);
