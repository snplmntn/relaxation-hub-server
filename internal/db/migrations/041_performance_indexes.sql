-- Migration 041: Performance & Security Optimizations
-- Implements high-priority audit recommendations

-- ============================================================================
-- 1. Missing Indexes for Therapist Availability Queries
-- ============================================================================

-- Index for therapist availability lookups (therapist schedule queries)
CREATE INDEX IF NOT EXISTS idx_bookings_therapist_scheduled 
ON bookings(therapist_id, scheduled_start) 
WHERE therapist_id IS NOT NULL AND status NOT IN ('cancelled', 'no_show');

-- Index for active offer lookup by therapist
CREATE INDEX IF NOT EXISTS idx_booking_offers_therapist_active 
ON booking_offers(therapist_id, status, expires_at) 
WHERE status = 'pending';

-- Covering index for live location fetches (avoids table lookup)
CREATE INDEX IF NOT EXISTS idx_live_locations_user_coords 
ON live_locations(user_id) 
INCLUDE (latitude, longitude, last_updated);

-- Index for booking group lookups (sequential bundling)
CREATE INDEX IF NOT EXISTS idx_bookings_group_id 
ON bookings(group_id) 
WHERE group_id IS NOT NULL;

-- ============================================================================
-- 2. Additional Query Optimization Indexes
-- ============================================================================

-- Index for extension request lookups by booking
CREATE INDEX IF NOT EXISTS idx_extension_requests_booking_status 
ON booking_extension_requests(booking_id, status);

-- Index for payment verification queue
CREATE INDEX IF NOT EXISTS idx_payments_pending_proof 
ON payments(status, proof_url) 
WHERE status = 'pending' AND proof_url IS NOT NULL;

-- Index for therapist service matching with pressure preferences
CREATE INDEX IF NOT EXISTS idx_therapist_services_all_pressures 
ON therapist_services(service_id, therapist_id) 
WHERE supports_soft = TRUE OR supports_moderate = TRUE OR supports_hard = TRUE;
