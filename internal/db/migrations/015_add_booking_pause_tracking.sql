-- Migration: Add pause tracking fields to bookings table
-- Allows tracking pause/resume cycles during sessions

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS total_paused_seconds INT DEFAULT 0;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS current_pause_start TIMESTAMPTZ;

-- Down migration (for rollback):
-- ALTER TABLE bookings DROP COLUMN IF EXISTS total_paused_seconds;
-- ALTER TABLE bookings DROP COLUMN IF EXISTS current_pause_start;
