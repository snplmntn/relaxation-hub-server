-- Durable due jobs for booking reminders.
-- Replaces recurring broad scans of assigned booking schedule windows.

CREATE TABLE IF NOT EXISTS booking_reminder_jobs (
    job_id BIGSERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    scheduled_start TIMESTAMP NOT NULL,
    due_at TIMESTAMP NOT NULL,
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (booking_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_booking_reminder_jobs_due_unprocessed
    ON booking_reminder_jobs (due_at, job_id)
    WHERE processed_at IS NULL;

CREATE OR REPLACE FUNCTION enqueue_booking_reminder_jobs()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'assigned' AND NEW.scheduled_start IS NOT NULL THEN
        INSERT INTO booking_reminder_jobs (booking_id, event_type, scheduled_start, due_at)
        SELECT NEW.booking_id, reminder.event_type, NEW.scheduled_start, NEW.scheduled_start - reminder.before_start
        FROM (VALUES
            ('reminder_24h'::text, INTERVAL '24 hours'),
            ('reminder_2h'::text, INTERVAL '2 hours')
        ) AS reminder(event_type, before_start)
        ON CONFLICT (booking_id, event_type) DO UPDATE
        SET scheduled_start = EXCLUDED.scheduled_start,
            due_at = EXCLUDED.due_at,
            processed_at = CASE
                WHEN booking_reminder_jobs.scheduled_start IS DISTINCT FROM EXCLUDED.scheduled_start THEN NULL
                ELSE booking_reminder_jobs.processed_at
            END,
            updated_at = NOW();
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enqueue_booking_reminder_jobs ON bookings;
CREATE TRIGGER trg_enqueue_booking_reminder_jobs
AFTER INSERT OR UPDATE OF status, scheduled_start ON bookings
FOR EACH ROW
EXECUTE FUNCTION enqueue_booking_reminder_jobs();
