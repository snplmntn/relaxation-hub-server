-- Migration: Add index for efficient therapist unassignment tracking
-- Used to query therapist_unassigned events by actor_id and event_type for daily/weekly limits

CREATE INDEX IF NOT EXISTS idx_booking_events_actor_type_time 
ON booking_events(actor_id, event_type, created_at);

-- Comment: This index supports the unassignment policy (3/day, 5/week limits)
-- by enabling efficient COUNT queries filtering on actor_id, event_type, and created_at range.
