-- Migration: Add performance indexes for high-load scenarios
-- This addresses audit finding: expensive queries in therapist matching and offer lookups

-- Index for booking conflict check subquery in therapist matching
-- Covers: SELECT 1 FROM bookings WHERE therapist_id = $1 AND status IN (...) AND scheduled_start ...
CREATE INDEX IF NOT EXISTS idx_bookings_therapist_schedule 
ON bookings (therapist_id, scheduled_start, status) 
WHERE status NOT IN ('cancelled', 'completed', 'no_show');

-- Index for active offers lookup by booking_id
-- Covers: SELECT ... FROM booking_offers WHERE booking_id = ANY($1) AND status = 'pending' AND expires_at > NOW()
CREATE INDEX IF NOT EXISTS idx_booking_offers_active 
ON booking_offers (booking_id, status, expires_at) 
WHERE status = 'pending';

-- Index for therapist booking counts query (used in fairness scoring)
-- Covers: SELECT therapist_id, COUNT(*) FROM bookings WHERE therapist_id = ANY($1) AND status = 'completed' AND created_at > $2
CREATE INDEX IF NOT EXISTS idx_bookings_therapist_completed 
ON bookings (therapist_id, created_at) 
WHERE status = 'completed';

-- Index for assignment queue processing (batch dequeue)
-- Corrected table name from assignment_queue to booking_assignment_queue
CREATE INDEX IF NOT EXISTS idx_booking_assignment_queue_next_attempt 
ON booking_assignment_queue (next_attempt_at) 
WHERE next_attempt_at IS NOT NULL;
