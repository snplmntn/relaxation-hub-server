-- Backfill reminder jobs for existing future assigned bookings.

INSERT INTO booking_reminder_jobs (booking_id, event_type, scheduled_start, due_at)
SELECT
    b.booking_id,
    reminder.event_type,
    b.scheduled_start,
    b.scheduled_start - reminder.before_start
FROM bookings b
CROSS JOIN (VALUES
    ('reminder_24h'::text, INTERVAL '24 hours'),
    ('reminder_2h'::text, INTERVAL '2 hours')
) AS reminder(event_type, before_start)
WHERE b.status = 'assigned'
  AND b.scheduled_start IS NOT NULL
  AND b.scheduled_start > NOW()
ON CONFLICT (booking_id, event_type) DO UPDATE
SET scheduled_start = EXCLUDED.scheduled_start,
    due_at = EXCLUDED.due_at,
    processed_at = CASE
        WHEN booking_reminder_jobs.scheduled_start IS DISTINCT FROM EXCLUDED.scheduled_start THEN NULL
        ELSE booking_reminder_jobs.processed_at
    END,
    updated_at = NOW();
