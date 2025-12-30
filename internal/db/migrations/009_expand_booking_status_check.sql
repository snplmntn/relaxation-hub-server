-- Migration: 009 - Expand bookings.status CHECK to include application statuses
-- Idempotent: drops existing constraint if present and recreates it.

BEGIN;

-- Drop old constraint if it exists (safe to run on upgraded DBs)
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;

-- Recreate constraint with the full set of statuses used by the application
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (
  status IN (
    'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed',
    'cancelled', 'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
  )
);

COMMIT;

-- Notes:
-- 1) This migration is intentionally minimal and idempotent.
-- 2) For existing production DBs, run this migration with normal tooling or via psql.
-- 3) If you use a migration runner, ensure this file is applied after existing migrations.
