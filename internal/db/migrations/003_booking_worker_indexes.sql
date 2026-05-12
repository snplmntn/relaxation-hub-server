-- Speeds up booking lifecycle workers and email/reminder duplicate checks.
CREATE INDEX IF NOT EXISTS idx_bookings_in_progress_actual_start
    ON bookings (actual_start)
    WHERE status = 'in_progress' AND actual_start IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_bookings_assigned_scheduled_start
    ON bookings (scheduled_start)
    WHERE status = 'assigned';

CREATE INDEX IF NOT EXISTS idx_booking_events_booking_type
    ON booking_events (booking_id, event_type);
