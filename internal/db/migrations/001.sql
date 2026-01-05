-- ============================================================================
-- CONSOLIDATED MIGRATION: Initial Schema + All Feature Migrations (001-018)
-- ============================================================================

-- ============================================================================
-- 1. CORE USER & IDENTITY SCHEMA
-- ============================================================================

-- Stores the core user profile, independent of login method
-- Soft deletion enabled via deleted_at
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    full_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('client', 'therapist', 'admin', 'super_admin')),
    
    -- These are for contact/display, NOT auth.
    -- They are set *after* an identity is verified.
    primary_email VARCHAR(100) UNIQUE,
    primary_phone VARCHAR(20),
    profile_photo TEXT,
    gender VARCHAR(20),
    emergency_contact_name VARCHAR(100),
    emergency_contact_phone VARCHAR(20),
    notification_preferences JSONB DEFAULT '{"push_notifications": true, "email_notifications": true, "sms_notifications": false, "booking_updates": true, "promotions": true, "rating_requests": true}'::jsonb,
    account_status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive')),
    fcm_token TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_role ON users(role) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_primary_email ON users(primary_email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_users_account_status ON users(account_status);
CREATE INDEX idx_users_fcm_token ON users(fcm_token) WHERE fcm_token IS NOT NULL;

CREATE TABLE user_auth_identities (
    identity_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    provider VARCHAR(30) NOT NULL,
    provider_key TEXT NOT NULL,
    password_hash TEXT,
    is_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_key)
);

CREATE INDEX idx_auth_identities_user ON user_auth_identities(user_id);
CREATE INDEX idx_auth_identities_provider_key ON user_auth_identities(provider, provider_key);

CREATE TABLE addresses (
    address_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    label VARCHAR(50),
    street_address VARCHAR(150) NOT NULL,
    barangay VARCHAR(100),
    city VARCHAR(100) NOT NULL,
    province VARCHAR(100),
    postal_code VARCHAR(20),
    landmark VARCHAR(200),
    country VARCHAR(100) DEFAULT 'Philippines',
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    is_default BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_addresses_user ON addresses(user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_one_default_address ON addresses(user_id) WHERE is_default = TRUE AND deleted_at IS NULL;

CREATE TABLE client_profiles (
    client_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    avg_rating NUMERIC(3,2) DEFAULT 0.0,
    total_reviews INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_client_profiles_rating ON client_profiles(avg_rating);

CREATE TABLE user_blocks (
    blocker_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    blocked_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blocker_user_id, blocked_user_id),
    CHECK (blocker_user_id != blocked_user_id)
);

CREATE INDEX idx_user_blocks_blocker ON user_blocks(blocker_user_id);
CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_user_id);

-- ============================================================================
-- 2. THERAPIST & SERVICE CATALOG SCHEMA
-- ============================================================================

CREATE TABLE branches (
    branch_id SERIAL PRIMARY KEY,
    branch_name VARCHAR(150) NOT NULL,
    address_line VARCHAR(255),
    barangay VARCHAR(100),
    city VARCHAR(100),
    province VARCHAR(100),
    postal_code VARCHAR(20),
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    contact_no VARCHAR(20),
    email VARCHAR(100),
    operating_hours JSONB,
    is_active BOOLEAN DEFAULT TRUE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_branches_active ON branches(is_active) WHERE deleted_at IS NULL;

CREATE TABLE services (
    service_id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    preview_image_url TEXT,
    base_price NUMERIC(10,2) NOT NULL,
    therapist_commission NUMERIC(10,2) DEFAULT 0,
    duration_minutes INT NOT NULL DEFAULT 60,
    is_active BOOLEAN DEFAULT TRUE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (base_price >= 0),
    CHECK (duration_minutes > 0)
);

CREATE INDEX idx_services_active ON services(service_id) WHERE deleted_at IS NULL;

CREATE TABLE therapist_profiles (
    therapist_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    branch_id INT REFERENCES branches(branch_id) ON DELETE SET NULL,
    bio TEXT,
    years_experience INT,
    avg_rating NUMERIC(3,2) DEFAULT 0.0,
    total_reviews INT DEFAULT 0,
    total_bookings INT DEFAULT 0,
    accept_assignments BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (years_experience >= 0)
);

CREATE INDEX idx_therapist_profiles_rating ON therapist_profiles(avg_rating);
CREATE INDEX idx_therapist_profiles_branch ON therapist_profiles(branch_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_therapist_profiles_accept_assignments ON therapist_profiles(accept_assignments);
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_verified ON therapist_profiles(is_verified) WHERE deleted_at IS NULL;

CREATE TABLE therapist_documents (
    document_id SERIAL PRIMARY KEY,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    document_url TEXT NOT NULL,
    document_type VARCHAR(50) CHECK (document_type IN ('Certification', 'ID', 'License')),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected')),
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMP,
    verified_by INT REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_therapist_documents_therapist ON therapist_documents(therapist_id);
CREATE INDEX idx_therapist_documents_status ON therapist_documents(status);

CREATE TABLE therapist_services (
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE RESTRICT,
    supports_soft BOOLEAN NOT NULL DEFAULT FALSE,
    supports_moderate BOOLEAN NOT NULL DEFAULT FALSE,
    supports_hard BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (therapist_id, service_id)
);

CREATE INDEX idx_therapist_services_service ON therapist_services(service_id);
-- Support flags for therapist_services (backfilled by migration 009 in upgrades)
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_soft ON therapist_services (service_id) WHERE supports_soft = TRUE;
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_moderate ON therapist_services (service_id) WHERE supports_moderate = TRUE;
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_hard ON therapist_services (service_id) WHERE supports_hard = TRUE;

CREATE TABLE IF NOT EXISTS favorite_therapists (
    user_id INT NOT NULL,
    therapist_id INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, therapist_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (therapist_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_favorite_therapists_user_id ON favorite_therapists(user_id);
-- ============================================================================
-- 3. PROMOTIONS & BOOKING SCHEMA
-- ============================================================================

-- Defines promotional codes with time-based rules
CREATE TABLE promotions (
    promo_id SERIAL PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    discount_percentage INT CHECK (discount_percentage BETWEEN 1 AND 100),
    discount_amount NUMERIC(10,2),
    
    valid_from TIMESTAMP,
    valid_until TIMESTAMP,
    max_uses INT,
    current_uses INT DEFAULT 0,
    
    -- Advanced time-based rules
    days_of_week INT[], -- e.g., [1,2,3,4,5] (Mon-Fri)
    start_time TIME, -- e.g., '15:00:00'
    end_time TIME, -- e.g., '19:00:00'
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Validate time ranges
    CHECK (end_time IS NULL OR start_time IS NULL OR end_time > start_time),
    CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from)
);

-- Indexes for promotions
CREATE INDEX idx_promotions_code ON promotions(code) WHERE deleted_at IS NULL;
CREATE INDEX idx_promotions_validity ON promotions(valid_from, valid_until) 
    WHERE deleted_at IS NULL;

-- Tracks which user has used which promo code
CREATE TABLE user_promotions (
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    promo_id INT REFERENCES promotions(promo_id) ON DELETE RESTRICT,
    times_used INT DEFAULT 1,
    PRIMARY KEY (user_id, promo_id),
    
    -- Ensure non-negative usage
    CHECK (times_used >= 0)
);

-- Index for user promotions
CREATE INDEX idx_user_promotions_user ON user_promotions(user_id);

-- Core booking table - links all entities together
CREATE TABLE bookings (
    booking_id SERIAL PRIMARY KEY,
    client_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE SET NULL,
    address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL,
    promo_id INT REFERENCES promotions(promo_id) ON DELETE SET NULL,
    
    reference_code VARCHAR(20),
    payment_method VARCHAR(20) CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer')) NOT NULL DEFAULT 'cash',
    
    gender_preference VARCHAR(10) CHECK (gender_preference IN ('male', 'female', 'any')),
    pressure_preference VARCHAR(10) CHECK (pressure_preference IN ('soft', 'medium', 'hard')),
    notes TEXT,
    
    duration_minutes INT NOT NULL,
    scheduled_start TIMESTAMP,
    actual_start TIMESTAMP,
    actual_end TIMESTAMP,
    
    raw_total NUMERIC(10,2),
    discount NUMERIC(10,2) DEFAULT 0,
    final_total NUMERIC(10,2),
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' 
        CHECK (status IN (
            'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed', 'cancelled',
            'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
        )),
    
    -- Migration 006: Additional timestamps and cancellation info
    assigned_at TIMESTAMP,
    therapist_arrived_at TIMESTAMP,
    no_show_at TIMESTAMP,
    cancelled_by VARCHAR(20),
    cancelled_at TIMESTAMP,
    cancellation_reason TEXT,

    -- Migration 015: Pause tracking
    total_paused_seconds INT DEFAULT 0,
    current_pause_start TIMESTAMPTZ,
    
    -- Migration 023: Extension wait time
    extension_wait_seconds INT DEFAULT 0,

    -- Migration 025: Payment breakdown
    payment_breakdown JSONB, -- Stores itemized price breakdown: base_price, duration_markup, extensions_total, service_snapshot_name

    -- Migration 028: Commission tracking
    therapist_earnings NUMERIC(10,2),
    platform_fee NUMERIC(10,2),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Data validation constraints
    CHECK (duration_minutes > 0),
    CHECK (final_total >= 0),
    CHECK (actual_end IS NULL OR actual_start IS NULL OR actual_end > actual_start),
    CHECK (discount >= 0),
    CHECK (raw_total >= 0)
);

-- Indexes for bookings
-- CRITICAL: These are the most queried fields
CREATE INDEX idx_bookings_client ON bookings(client_id);
CREATE INDEX idx_bookings_therapist ON bookings(therapist_id);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_scheduled_start ON bookings(scheduled_start);
CREATE INDEX idx_bookings_created_at ON bookings(created_at DESC);
-- Composite index for finding available bookings
CREATE INDEX idx_bookings_composite ON bookings(status, scheduled_start) 
    WHERE status = 'pending';
-- Unique index for reference_code lookup
CREATE UNIQUE INDEX idx_bookings_reference_code ON bookings(reference_code) WHERE reference_code IS NOT NULL;

-- Ensure bookings can be created without an immediate therapist.
-- This makes `therapist_id` nullable for fresh DBs and is safe to run
-- idempotently on existing DBs.
ALTER TABLE IF EXISTS bookings ALTER COLUMN therapist_id DROP NOT NULL;

-- Durable assignment queue for bookings created without an immediate therapist
CREATE TABLE IF NOT EXISTS booking_assignment_queue (
    queue_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE CASCADE,
    enqueued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    attempts INT DEFAULT 0,
    last_attempt_at TIMESTAMP,
    next_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- ensure a booking only has one queue row at a time
    UNIQUE (booking_id)
);

CREATE INDEX IF NOT EXISTS idx_booking_assignment_queue_next_attempt ON booking_assignment_queue(next_attempt_at);

-- Migration 006: Booking Events
CREATE TABLE IF NOT EXISTS booking_events (
    event_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    actor_id INT REFERENCES users(user_id) ON DELETE SET NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_booking_events_booking ON booking_events(booking_id);
CREATE INDEX IF NOT EXISTS idx_booking_events_type ON booking_events(event_type);
CREATE INDEX IF NOT EXISTS idx_booking_events_created_at ON booking_events(created_at DESC);

-- Migration 008: Booking Offers
CREATE TABLE IF NOT EXISTS booking_offers (
    offer_id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    therapist_id BIGINT NOT NULL REFERENCES users(user_id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, accepted, declined, expired
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE(booking_id, therapist_id)
);

CREATE INDEX IF NOT EXISTS idx_booking_offers_booking_id ON booking_offers(booking_id);
CREATE INDEX IF NOT EXISTS idx_booking_offers_therapist_id ON booking_offers(therapist_id);
CREATE INDEX IF NOT EXISTS idx_booking_offers_status ON booking_offers(status);

-- Migration 022: Booking Extension Requests (Request-Approval Flow)
CREATE TABLE IF NOT EXISTS booking_extension_requests (
    request_id SERIAL PRIMARY KEY,
    booking_id INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    requested_minutes INTEGER NOT NULL CHECK (requested_minutes > 0),
    additional_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    requested_by INTEGER REFERENCES users(user_id),
    responded_by INTEGER REFERENCES users(user_id),
    response_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_extension_requests_booking ON booking_extension_requests(booking_id);
CREATE INDEX IF NOT EXISTS idx_extension_requests_pending ON booking_extension_requests(status) WHERE status = 'pending';

-- ============================================================================
-- 4. PAYMENT SCHEMA (Option B - Separate payments table for Xendit)
-- ============================================================================

-- Payments for bookings - handles Xendit integration
CREATE TABLE payments (
    payment_id SERIAL PRIMARY KEY,
    booking_id INT UNIQUE REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    
    amount NUMERIC(10,2) NOT NULL,
    
    -- Payment gateway details
    gateway VARCHAR(50) NOT NULL, -- 'xendit', 'gcash', 'paymaya', 'cash'
    transaction_id VARCHAR(100), -- Xendit's unique transaction ID
    
    -- Payment status
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'expired')),
    
    -- Store complete Xendit response for audit trail
    -- Includes: payment_id, status, failure_code, authorization_data, etc.
    gateway_response JSONB,
    
    -- Xendit webhook verification
    webhook_id VARCHAR(100), -- For idempotency
    
    transaction_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    paid_at TIMESTAMP,
    refunded_at TIMESTAMP,
    
    -- Migration 023: Payment Proof
    proof_url TEXT,
    
    -- Migration 024: Payment Verification
    verified_at TIMESTAMPTZ,
    verified_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    
    -- Ensure positive amount
    CHECK (amount >= 0)
);

-- Indexes for payments
CREATE INDEX idx_payments_booking ON payments(booking_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_transaction_id ON payments(transaction_id);
CREATE INDEX idx_payments_gateway ON payments(gateway);
-- For querying Xendit responses
CREATE INDEX idx_payments_gateway_response ON payments USING GIN (gateway_response);
CREATE INDEX IF NOT EXISTS idx_payments_proof_url ON payments(proof_url) WHERE proof_url IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_verified ON payments(verified_at) WHERE verified_at IS NOT NULL;

-- ============================================================================
-- 5. REVIEWS & RATINGS SCHEMA
-- ============================================================================

-- Client's review of the session
CREATE TABLE reviews (
    review_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    client_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE RESTRICT,
    
    therapist_rating INT CHECK (therapist_rating BETWEEN 1 AND 5),
    therapist_review TEXT,
    
    service_rating INT CHECK (service_rating BETWEEN 1 AND 5),
    service_review TEXT,
    
    platform_rating INT CHECK (platform_rating BETWEEN 1 AND 5),
    platform_review TEXT,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for reviews
CREATE INDEX idx_reviews_therapist ON reviews(therapist_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_client ON reviews(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_booking ON reviews(booking_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_created_at ON reviews(created_at DESC);

-- Migration 018: Prevent duplicate reviews for the same booking
CREATE UNIQUE INDEX idx_reviews_unique_booking_non_deleted 
ON reviews(booking_id) 
WHERE deleted_at IS NULL;


-- Therapist's review of the client
CREATE TABLE client_reviews (
    client_review_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    client_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    client_rating INT NOT NULL CHECK (client_rating BETWEEN 1 AND 5),
    client_review TEXT,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for client reviews
CREATE INDEX idx_client_reviews_client ON client_reviews(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_client_reviews_therapist ON client_reviews(therapist_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_client_reviews_booking ON client_reviews(booking_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- 6. REAL-TIME FEATURES SCHEMA
-- ============================================================================

-- Real-time location tracking (for live map)
CREATE TABLE live_locations (
    location_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    latitude NUMERIC(9,6) NOT NULL,
    longitude NUMERIC(9,6) NOT NULL,
    
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE (user_id) -- one active row per user
);

-- Indexes for live locations
-- CRITICAL: For nearby therapist search
CREATE INDEX idx_live_locations_coords ON live_locations(latitude, longitude);
CREATE INDEX idx_live_locations_updated ON live_locations(last_updated);

-- ============================================================================
-- 7. MESSAGING SCHEMA
-- ============================================================================

-- Chat: A thread between participants
CREATE TABLE conversations (
    conversation_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    is_group BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    ,updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for conversations
CREATE INDEX idx_conversations_booking ON conversations(booking_id);

-- Chat: Links users to a conversation
CREATE TABLE conversation_participants (
    conversation_id INT REFERENCES conversations(conversation_id) ON DELETE RESTRICT,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    role VARCHAR(20) DEFAULT 'member', -- 'client', 'therapist', 'admin'
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (conversation_id, user_id)
);

-- Indexes for conversation participants
CREATE INDEX idx_conversation_participants_user ON conversation_participants(user_id, conversation_id);

-- Chat: The actual messages
CREATE TABLE messages (
    message_id SERIAL PRIMARY KEY,
    conversation_id INT REFERENCES conversations(conversation_id) ON DELETE RESTRICT,
    sender_id INT REFERENCES users(user_id) ON DELETE RESTRICT,

    content TEXT,
    message_type VARCHAR(20) DEFAULT 'text', -- 'text', 'image', 'system'
    media_url TEXT,
    read_at TIMESTAMP,
    deleted_at TIMESTAMP,

    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for messages
-- CRITICAL: For chat performance
CREATE INDEX idx_messages_conversation ON messages(conversation_id, sent_at);
CREATE INDEX idx_messages_unread ON messages(conversation_id) 
    WHERE read_at IS NULL;
CREATE INDEX idx_messages_sender ON messages(sender_id);

-- Logs for the in-app call feature
CREATE TABLE call_logs (
    call_id SERIAL PRIMARY KEY,
    conversation_id INT REFERENCES conversations(conversation_id) ON DELETE RESTRICT,
    initiator_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP,
    status VARCHAR(20) CHECK (status IN ('missed', 'connected', 'declined')),
    
    -- Ensure valid call duration
    CHECK (end_time IS NULL OR end_time > start_time)
);

-- Indexes for call logs
CREATE INDEX idx_call_logs_conversation ON call_logs(conversation_id);
CREATE INDEX idx_call_logs_initiator ON call_logs(initiator_id);

-- ============================================================================
-- 8. SAFETY & EMERGENCY SCHEMA
-- ============================================================================

-- Logs for the emergency button
CREATE TABLE emergency_alerts (
    alert_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    triggered_by INT REFERENCES users(user_id) ON DELETE RESTRICT,
    alert_type VARCHAR(50),
    description TEXT,
    
    triggered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    location_lat NUMERIC(9,6),
    location_lng NUMERIC(9,6),
    
    -- Enhanced status tracking
    status VARCHAR(20) DEFAULT 'pending' 
        CHECK (status IN ('pending', 'acknowledged', 'resolved', 'false_alarm')),
    
    resolved BOOLEAN DEFAULT FALSE, -- Kept for backward compatibility
    resolved_at TIMESTAMP,
    resolved_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    resolution_notes TEXT
);

-- Indexes for emergency alerts
CREATE INDEX idx_emergency_alerts_status ON emergency_alerts(status);
CREATE INDEX idx_emergency_alerts_booking ON emergency_alerts(booking_id);
CREATE INDEX idx_emergency_alerts_triggered_by ON emergency_alerts(triggered_by);
CREATE INDEX idx_emergency_alerts_triggered_at ON emergency_alerts(triggered_at DESC);

-- ============================================================================
-- 9. ADMIN & AUDIT SCHEMA
-- ============================================================================

-- Audit log for admin actions
CREATE TABLE admin_actions (
    action_id SERIAL PRIMARY KEY,
    admin_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT, -- The user being acted upon
    
    action_type VARCHAR(50) NOT NULL, -- e.g., 'warning', 'ban', 'refund'
    details TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for admin actions
CREATE INDEX idx_admin_actions_admin ON admin_actions(admin_id);
CREATE INDEX idx_admin_actions_user ON admin_actions(user_id);
CREATE INDEX idx_admin_actions_type ON admin_actions(action_type);
CREATE INDEX idx_admin_actions_created_at ON admin_actions(created_at DESC);

-- ============================================================================
-- 10. ADDITIONAL FEATURES
-- ============================================================================

-- Notification history (was missing in original schema)
CREATE TABLE notifications (
    notification_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    type VARCHAR(50) NOT NULL, -- 'payment_succeeded', 'therapist_arrived', etc.
    title VARCHAR(200),
    message TEXT,
    data JSONB, -- Additional structured data
    
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for notifications
CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_unread ON notifications(user_id, is_read) 
    WHERE is_read = FALSE;
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);

-- Referral tracking (for growth features)
CREATE TABLE referrals (
    referral_id SERIAL PRIMARY KEY,
    referrer_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    referee_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    reward_amount NUMERIC(10,2),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure positive reward
    CHECK (reward_amount >= 0)
);

-- Indexes for referrals
CREATE INDEX idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX idx_referrals_referee ON referrals(referee_id);

-- ============================================================================
-- 10. SUPPORT & TICKETING SCHEMA
-- ============================================================================

-- Defines the support tickets submitted by users
CREATE TABLE support_tickets (
    ticket_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE SET NULL, -- Nullable if user deletes account but we keep ticket
    
    -- Contact Information
    full_name VARCHAR(150),
    connected_email_phone VARCHAR(150), -- Snapshot of profile info at time of creation
    contact_email_phone VARCHAR(150),   -- User provided contact info
    
    -- Ticket Details
    category VARCHAR(50) NOT NULL CHECK (category IN (
        'Booking Issue',
        'Payment & Billing Issue',
        'Safety & Conduct Report',
        'Technical Issue (App Bug)',
        'Account & Profile Support',
        'General Inquiry & Feedback',
        'Other'
    )),
    
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL, -- Conditional field
    description TEXT NOT NULL,
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'investigating', 'resolved', 'closed')),
        
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient lookup
CREATE INDEX idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX idx_support_tickets_status ON support_tickets(status);
CREATE INDEX idx_support_tickets_booking ON support_tickets(booking_id);
CREATE INDEX idx_support_tickets_created_at ON support_tickets(created_at DESC);

-- Attachments for tickets (Images/Screenshots)
CREATE TABLE support_ticket_attachments (
    attachment_id SERIAL PRIMARY KEY,
    ticket_id INT REFERENCES support_tickets(ticket_id) ON DELETE CASCADE,
    file_url TEXT NOT NULL,
    file_type VARCHAR(50) DEFAULT 'image',
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_support_ticket_attachments_ticket ON support_ticket_attachments(ticket_id);

-- ============================================================================
-- 11. TRIGGERS & FUNCTIONS
-- ============================================================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to users table
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- Apply to other tables that have `updated_at`
CREATE TRIGGER update_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_branches_updated_at
    BEFORE UPDATE ON branches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_services_updated_at
    BEFORE UPDATE ON services
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_promotions_updated_at
    BEFORE UPDATE ON promotions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_therapist_profiles_updated_at
    BEFORE UPDATE ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reviews_updated_at
    BEFORE UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_client_reviews_updated_at
    BEFORE UPDATE ON client_reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_bookings_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_support_tickets_updated_at
    BEFORE UPDATE ON support_tickets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Auto-update therapist rating
CREATE OR REPLACE FUNCTION update_therapist_rating()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE therapist_profiles
    SET 
        avg_rating = (
            SELECT COALESCE(AVG(therapist_rating), 0)
            FROM reviews
            WHERE therapist_id = NEW.therapist_id AND deleted_at IS NULL
        ),
        total_reviews = (
            SELECT COUNT(*)
            FROM reviews
            WHERE therapist_id = NEW.therapist_id AND deleted_at IS NULL
        )
    WHERE therapist_id = NEW.therapist_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_therapist_rating
    AFTER INSERT OR UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_therapist_rating();

-- Auto-update client rating
CREATE OR REPLACE FUNCTION update_client_rating()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE client_profiles
    SET 
        avg_rating = (
            SELECT COALESCE(AVG(client_rating), 0)
            FROM client_reviews
            WHERE client_id = NEW.client_id AND deleted_at IS NULL
        ),
        total_reviews = (
            SELECT COUNT(*)
            FROM client_reviews
            WHERE client_id = NEW.client_id AND deleted_at IS NULL
        )
    WHERE client_id = NEW.client_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_client_rating
    AFTER INSERT OR UPDATE ON client_reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_client_rating();

-- ============================================================================
-- 5. RATE LIMITING SCHEMA
-- ============================================================================

-- Create rate limit tracking table for auth endpoints
CREATE TABLE auth_rate_limits (
    id SERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL, -- Email or phone/IP address
    attempt_count INT DEFAULT 1,
    first_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    locked_until TIMESTAMP,
    
    UNIQUE (identifier),
    CHECK (attempt_count >= 0)
);

-- Indexes for rate limit lookups
CREATE INDEX idx_auth_rate_limits_identifier ON auth_rate_limits(identifier);
CREATE INDEX idx_auth_rate_limits_locked_until ON auth_rate_limits(locked_until);
-- ============================================================================
-- 12. HELPFUL COMMENTS & DOCUMENTATION
-- ============================================================================

COMMENT ON TABLE users IS 'Core user accounts for clients, therapists, and admins. Soft deletion enabled.';
COMMENT ON TABLE user_auth_identities IS 'Supports multiple auth providers (email, phone, Google, Apple) per user.';
COMMENT ON TABLE bookings IS 'Central transaction table linking clients, therapists, services, and addresses.';
COMMENT ON TABLE payments IS 'Xendit payment integration with full webhook response storage in JSONB.';
COMMENT ON TABLE live_locations IS 'Real-time GPS tracking for therapist navigation. One row per active user.';
COMMENT ON TABLE emergency_alerts IS 'Safety feature for panic button with status tracking and resolution notes.';
COMMENT ON TABLE therapist_profiles IS 'Extended therapist data with cached ratings for performance.';
COMMENT ON COLUMN addresses.latitude IS 'NUMERIC(9,6) provides ~11cm accuracy. PH coordinates: lat 5-20, lon 116-127.';
COMMENT ON COLUMN payments.gateway_response IS 'Stores complete Xendit webhook payload including transaction_id, status, failure codes, and authorization data.';

-- NOTE: Booking event types (used by booking_events table/migration 006) include values such as
-- 'created', 'assigned', 'therapist_arrived', 'confirm_start', 'payment_succeeded', 'no_show', 'cancelled'.
-- The project normalizes start confirmations to the single 'confirm_start' event. See migration 007_migrate_confirm_start.sql.

-- ============================================================================
-- DATA MIGRATIONS (from 002_migrate_messages.sql & 007_migrate_confirm_start.sql)
-- These are idempotent, safe to run when consolidating initial schema and
-- should be no-ops on freshly created databases that already use the final
-- schema. Keep them at the end of the consolidated migration.
-- ============================================================================

-- 1) Messages structural + data migration
BEGIN;

DO $$
BEGIN
    -- Rename legacy message_text -> content
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'message_text'
    ) THEN
        EXECUTE 'ALTER TABLE messages RENAME COLUMN message_text TO content';
    END IF;

    -- Rename legacy created_at -> sent_at
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'created_at'
    ) THEN
        EXECUTE 'ALTER TABLE messages RENAME COLUMN created_at TO sent_at';
    END IF;

    -- Add read_at and deleted_at if missing
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'read_at'
    ) THEN
        EXECUTE 'ALTER TABLE messages ADD COLUMN read_at TIMESTAMP';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE messages ADD COLUMN deleted_at TIMESTAMP';
    END IF;

    -- Migrate boolean is_read -> timestamp read_at when applicable
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'is_read'
    ) THEN
        -- mark read_at to sent_at for rows that were marked read
        EXECUTE 'UPDATE messages SET read_at = sent_at WHERE is_read = TRUE';
        EXECUTE 'ALTER TABLE messages DROP COLUMN is_read';
    END IF;

END;
$$;

-- Recreate / normalize message indexes used by application
DROP INDEX IF EXISTS idx_messages_conversation;
DROP INDEX IF EXISTS idx_messages_unread;
DROP INDEX IF EXISTS idx_messages_sender;

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(conversation_id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);

COMMIT;


-- 2) Normalize booking_events confirm_start event types (from migration 007)
BEGIN;

UPDATE booking_events
SET event_type = 'confirm_start'
WHERE event_type IN ('client_confirm_start', 'therapist_confirm_start', 'admin_confirm_start');

COMMIT;

-- Migration 009: payment_method is now included in the CREATE TABLE statement above
-- This section is kept for backward compatibility with existing databases
-- but is a no-op for fresh databases
BEGIN;

ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20)
        CHECK (payment_method IN ('cash', 'gcash'))
        NOT NULL DEFAULT 'cash';

COMMIT;


