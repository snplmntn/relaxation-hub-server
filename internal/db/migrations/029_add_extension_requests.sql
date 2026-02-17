-- Migration: Add booking_extension_requests table for request-approval flow
-- This table stores pending extension requests from clients that require therapist/admin approval

CREATE TABLE IF NOT EXISTS booking_extension_requests (
    request_id SERIAL PRIMARY KEY,
    booking_id INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    requested_minutes INTEGER NOT NULL CHECK (requested_minutes > 0),
    additional_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    requested_by INTEGER REFERENCES users(user_id),
    responded_by INTEGER REFERENCES users(user_id),
    response_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for quick lookups by booking
CREATE INDEX IF NOT EXISTS idx_extension_requests_booking ON booking_extension_requests(booking_id);

-- Index for pending requests (common query pattern)
CREATE INDEX IF NOT EXISTS idx_extension_requests_pending ON booking_extension_requests(status) WHERE status = 'pending';
