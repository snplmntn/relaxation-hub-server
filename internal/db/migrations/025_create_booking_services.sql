-- Multiple services per booking (one therapist, one block in the day-view).
-- The primary service still lives in bookings.service_id for backward compatibility.
-- This table stores the ordered list with price/duration snapshots so historical
-- records remain stable if a service's price later changes.
CREATE TABLE IF NOT EXISTS booking_services (
    booking_service_id SERIAL PRIMARY KEY,
    booking_id         INT NOT NULL REFERENCES bookings(booking_id)  ON DELETE CASCADE,
    service_id         INT NOT NULL REFERENCES services(service_id)  ON DELETE RESTRICT,
    position           INT NOT NULL DEFAULT 0,       -- display order within the booking
    price_snapshot     NUMERIC(10,2) NOT NULL,       -- base_price at time of booking
    duration_snapshot  INT NOT NULL,                 -- duration_minutes at time of booking
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_booking_services_booking_id ON booking_services(booking_id);
