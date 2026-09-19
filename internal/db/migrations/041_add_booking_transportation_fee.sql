ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS transportation_fee NUMERIC(10,2) NOT NULL DEFAULT 0
    CHECK (transportation_fee >= 0);

ALTER TABLE recurring_bookings
    ADD COLUMN IF NOT EXISTS transportation_fee NUMERIC(10,2) NOT NULL DEFAULT 0
    CHECK (transportation_fee >= 0);
