-- 004: Make therapist_id nullable and add booking assignment queue

ALTER TABLE bookings ALTER COLUMN therapist_id DROP NOT NULL;

-- Create a durable queue table for booking assignments
CREATE TABLE IF NOT EXISTS booking_assignment_queue (
    queue_id SERIAL PRIMARY KEY,
    booking_id INT UNIQUE REFERENCES bookings(booking_id) ON DELETE CASCADE,
    enqueued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    attempts INT DEFAULT 0,
    last_attempt_at TIMESTAMP,
    next_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_booking_assignment_queue_enqueued_at ON booking_assignment_queue(enqueued_at);
