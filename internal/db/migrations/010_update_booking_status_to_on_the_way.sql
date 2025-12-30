-- Migration: 010 - Update booking status from 'en_route' to 'on_the_way'
-- Description: Renames the internal status label and updates the check constraint.

BEGIN;

-- 1. Drop the old constraint (from 001.sql or 009.sql)
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;

-- 2. Update existing data to the new status labels
UPDATE bookings SET status = 'on_the_way' WHERE status = 'en_route';
UPDATE bookings SET status = 'pending' WHERE status = 'confirmed'; -- Map confirmed to pending

-- 3. Add the new constraint with 'on_the_way' and WITHOUT 'confirmed'
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (
  status IN (
    'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed',
    'cancelled', 'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
  )
);

COMMIT;
