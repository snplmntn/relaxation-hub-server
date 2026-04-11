-- Migration 070: Add indexes for high-traffic repository query paths.

-- bookings: client/therapist history pages and status-based admin listings
CREATE INDEX IF NOT EXISTS idx_bookings_client_created
    ON bookings (client_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bookings_therapist_created
    ON bookings (therapist_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bookings_status_created
    ON bookings (status, created_at DESC);

-- bookings: completed earnings rollups by therapist and end time window
CREATE INDEX IF NOT EXISTS idx_bookings_completed_end
    ON bookings (therapist_id, actual_end)
    WHERE status = 'completed' AND actual_end IS NOT NULL;

-- bookings: global pending queue ordered by creation time
CREATE INDEX IF NOT EXISTS idx_bookings_pending_unassigned
    ON bookings (created_at)
    WHERE status = 'pending' AND therapist_id IS NULL;

-- bookings/users: admin free-text search paths
CREATE INDEX IF NOT EXISTS idx_bookings_ref_trgm
    ON bookings USING GIN (reference_code gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_users_full_name_trgm
    ON users USING GIN (full_name gin_trgm_ops);

-- booking events: paginated listing filters
CREATE INDEX IF NOT EXISTS idx_booking_events_type_created
    ON booking_events (event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_booking_events_actor_created
    ON booking_events (actor_id, created_at DESC);

-- rides: dispatch queue and rider status lookups
CREATE INDEX IF NOT EXISTS idx_rides_status_created
    ON rides (status, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_rides_rider_status
    ON rides (rider_id, status);

CREATE INDEX IF NOT EXISTS idx_rides_booking
    ON rides (booking_id);

-- emergency alerts: status list/count sorted by trigger time
CREATE INDEX IF NOT EXISTS idx_emergency_alerts_status_time
    ON emergency_alerts (status, triggered_at DESC);

-- ledger summaries: active entries by date range
CREATE INDEX IF NOT EXISTS idx_ledger_entries_date_active
    ON ledger_entries (entry_date)
    WHERE voided = FALSE;
