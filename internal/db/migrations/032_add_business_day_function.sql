-- One definition of "which trading day does this booking belong to".
--
-- The spa opens at 1 PM and the last Day View slot starts 3:45 AM, so a
-- business day straddles two calendar dates. The reports key business_date on
-- DATE(actual_end), the calendar date a session *finished*, which splits one
-- night's takings across two sheets: a booking that starts 10 PM and ends
-- 12:14 AM lands on the following day. The Day View, the CSV export and the
-- accounting sheet all attribute by the day the session started, so the same
-- night reads differently depending on which page you open.
--
-- Booking timestamps are stored naive in UTC. Philippine Standard Time is a
-- fixed UTC+8 with no daylight saving and the day rolls over at 04:00 Manila,
-- so the whole rule collapses to (utc + 8h - 4h) = (utc + 4h) truncated to a
-- date. Keeping it to interval arithmetic is deliberate: AT TIME ZONE with a
-- named zone is only STABLE, which cannot be indexed, whereas this is
-- IMMUTABLE. Verified against all 617 existing bookings to give the same
-- answer as the timezone-aware form.
--
-- Boundaries, in Manila terms:
--   12:59 -> that calendar day   (before opening; off-hours work still lands
--                                 on a sheet rather than vanishing)
--   13:00 -> that calendar day   (opening)
--   03:45 -> the previous day    (last bookable slot)
--   03:59 -> the previous day
--   04:00 -> that calendar day   (rollover)
--
-- Every statement is independently re-runnable; there is no down migration.

CREATE OR REPLACE FUNCTION business_day(utc_ts TIMESTAMP)
RETURNS DATE
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
RETURNS NULL ON NULL INPUT
AS $$
    SELECT (utc_ts + INTERVAL '4 hours')::date;
$$;

COMMENT ON FUNCTION business_day(TIMESTAMP) IS
    'Trading day (13:00-04:00 Manila) that owns a UTC booking timestamp. '
    'Single source of truth for business_date across sales, payroll and exports.';

-- Reports filter on the business day of scheduled_start, so index the
-- expression rather than the raw column.
CREATE INDEX IF NOT EXISTS idx_bookings_business_day
    ON bookings (business_day(scheduled_start))
    WHERE scheduled_start IS NOT NULL;
