ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS booking_source VARCHAR(32) NOT NULL DEFAULT 'staff_web';

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS bookings_booking_source_check;

ALTER TABLE bookings
    ADD CONSTRAINT bookings_booking_source_check
    CHECK (booking_source IN ('hiraya_web', 'staff_web', 'client_app', 'customer'));

-- Before booking_source existed, the durable distinction available for legacy
-- records was the actor on the creation event. A client creating their own
-- booking corresponds to the Hiraya customer flow used in production.
UPDATE bookings AS booking
SET booking_source = 'hiraya_web'
WHERE EXISTS (
    SELECT 1
    FROM booking_events AS event
    WHERE event.booking_id = booking.booking_id
      AND event.event_type = 'created'
      AND event.actor_id = booking.client_id
)
AND NOT EXISTS (
    SELECT 1
    FROM booking_events AS event
    WHERE event.booking_id = booking.booking_id
      AND event.event_type = 'admin_created_booking'
);

CREATE INDEX IF NOT EXISTS idx_bookings_booking_source_scheduled_start
    ON bookings (booking_source, scheduled_start);
