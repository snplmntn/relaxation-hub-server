-- Consolidated Migration Script
-- ============================================================================
-- CONSOLIDATED MIGRATION: Initial Schema + All Feature Migrations (001-018)
-- ============================================================================

-- Enable PostGIS extension for geolocation features (rider module)
CREATE EXTENSION IF NOT EXISTS postgis;

-- ============================================================================
-- 1. CORE USER & IDENTITY SCHEMA
-- ============================================================================

-- Stores the core user profile, independent of login method
-- Soft deletion enabled via deleted_at
CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    full_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('client', 'therapist', 'admin', 'super_admin', 'rider')),
    
    -- These are for contact/display, NOT auth.
    -- They are set *after* an identity is verified.
    primary_email VARCHAR(100) UNIQUE,
    primary_phone VARCHAR(20),
    profile_photo TEXT,
    gender VARCHAR(20),
    emergency_contact_name VARCHAR(100),
    emergency_contact_phone VARCHAR(20),
    notification_preferences JSONB DEFAULT '{"push_notifications": true, "email_notifications": true, "sms_notifications": false, "booking_updates": true, "promotions": true, "rating_requests": true}'::jsonb,
    account_status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive', 'blocked')),
    status_reason TEXT,
    is_vip BOOLEAN NOT NULL DEFAULT FALSE,
    fcm_token TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_primary_email ON users(primary_email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_account_status ON users(account_status);
CREATE INDEX IF NOT EXISTS idx_users_non_active_status ON users(account_status) WHERE account_status != 'active';
CREATE INDEX IF NOT EXISTS idx_users_is_vip ON users(is_vip) WHERE is_vip = TRUE;
CREATE INDEX IF NOT EXISTS idx_users_fcm_token ON users(fcm_token) WHERE fcm_token IS NOT NULL;

CREATE TABLE IF NOT EXISTS user_auth_identities (
    identity_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    provider VARCHAR(30) NOT NULL,
    provider_key TEXT NOT NULL,
    password_hash TEXT,
    is_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_key)
);

CREATE INDEX IF NOT EXISTS idx_auth_identities_user ON user_auth_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_identities_provider_key ON user_auth_identities(provider, provider_key);

CREATE TABLE IF NOT EXISTS addresses (
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

CREATE INDEX IF NOT EXISTS idx_addresses_user ON addresses(user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_one_default_address ON addresses(user_id) WHERE is_default = TRUE AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS client_profiles (
    client_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    avg_rating NUMERIC(3,2) DEFAULT 0.0,
    total_reviews INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_client_profiles_rating ON client_profiles(avg_rating);

CREATE TABLE IF NOT EXISTS user_blocks (
    blocker_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    blocked_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blocker_user_id, blocked_user_id),
    CHECK (blocker_user_id != blocked_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_blocks_blocker ON user_blocks(blocker_user_id);
CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked ON user_blocks(blocked_user_id);

-- ============================================================================
-- 2. THERAPIST & SERVICE CATALOG SCHEMA
-- ============================================================================

CREATE TABLE IF NOT EXISTS branches (
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

CREATE INDEX IF NOT EXISTS idx_branches_active ON branches(is_active) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS services (
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

CREATE INDEX IF NOT EXISTS idx_services_active ON services(service_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS therapist_profiles (
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

CREATE INDEX IF NOT EXISTS idx_therapist_profiles_rating ON therapist_profiles(avg_rating);
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_branch ON therapist_profiles(branch_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_accept_assignments ON therapist_profiles(accept_assignments);
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_verified ON therapist_profiles(is_verified) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS therapist_documents (
    document_id SERIAL PRIMARY KEY,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    document_url TEXT NOT NULL,
    document_type VARCHAR(50) CHECK (document_type IN ('Certification', 'ID', 'License')),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected')),
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMP,
    verified_by INT REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_therapist_documents_therapist ON therapist_documents(therapist_id);
CREATE INDEX IF NOT EXISTS idx_therapist_documents_status ON therapist_documents(status);

CREATE TABLE IF NOT EXISTS therapist_services (
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE CASCADE,
    supports_soft BOOLEAN NOT NULL DEFAULT FALSE,
    supports_moderate BOOLEAN NOT NULL DEFAULT FALSE,
    supports_hard BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (therapist_id, service_id)
);

CREATE INDEX IF NOT EXISTS idx_therapist_services_service ON therapist_services(service_id);
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

CREATE INDEX IF NOT EXISTS idx_favorite_therapists_user_id ON favorite_therapists(user_id);
-- ============================================================================
-- 3. PROMOTIONS & BOOKING SCHEMA
-- ============================================================================

-- Defines promotional codes with time-based rules
CREATE TABLE IF NOT EXISTS promotions (
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
CREATE INDEX IF NOT EXISTS idx_promotions_code ON promotions(code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_promotions_validity ON promotions(valid_from, valid_until) 
    WHERE deleted_at IS NULL;

-- Tracks which user has used which promo code
CREATE TABLE IF NOT EXISTS user_promotions (
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    promo_id INT REFERENCES promotions(promo_id) ON DELETE RESTRICT,
    times_used INT DEFAULT 1,
    PRIMARY KEY (user_id, promo_id),
    
    -- Ensure non-negative usage
    CHECK (times_used >= 0)
);

-- Index for user promotions
CREATE INDEX IF NOT EXISTS idx_user_promotions_user ON user_promotions(user_id);

-- Booking groups must exist before bookings can reference them.
CREATE TABLE IF NOT EXISTS booking_groups (
    group_id SERIAL PRIMARY KEY,
    client_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL,
    scheduled_start TIMESTAMPTZ,
    raw_total NUMERIC(10,2) DEFAULT 0,
    discount NUMERIC(10,2) DEFAULT 0,
    final_total NUMERIC(10,2) DEFAULT 0,
    payment_method VARCHAR(20) CHECK (payment_method IN ('cash', 'gcash', 'bdo')),
    status VARCHAR(30) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled', 'paid')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_booking_groups_client_id ON booking_groups(client_id);

-- Core booking table - links all entities together
CREATE TABLE IF NOT EXISTS bookings (
    booking_id SERIAL PRIMARY KEY,
    client_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE SET NULL,
    address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL,
    promo_id INT REFERENCES promotions(promo_id) ON DELETE SET NULL,
    
    reference_code VARCHAR(20),
    payment_method VARCHAR(20) CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer')) NOT NULL DEFAULT 'cash',
    change_for DECIMAL(10, 2),
    
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

    -- Migration 033: Complex bookings support
    group_id INT REFERENCES booking_groups(group_id) ON DELETE SET NULL,
    guest_name VARCHAR(100),
    sequence_number INT DEFAULT 0,
    start_condition VARCHAR(30) DEFAULT 'fixed_time'
        CHECK (start_condition IN ('fixed_time', 'after_previous')),

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
CREATE INDEX IF NOT EXISTS idx_bookings_client ON bookings(client_id);
CREATE INDEX IF NOT EXISTS idx_bookings_therapist ON bookings(therapist_id);
CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status);
CREATE INDEX IF NOT EXISTS idx_bookings_scheduled_start ON bookings(scheduled_start);
CREATE INDEX IF NOT EXISTS idx_bookings_created_at ON bookings(created_at DESC);
-- Composite index for finding available bookings
CREATE INDEX IF NOT EXISTS idx_bookings_composite ON bookings(status, scheduled_start) 
    WHERE status = 'pending';
-- Unique index for reference_code lookup
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_reference_code ON bookings(reference_code) WHERE reference_code IS NOT NULL;
-- Index for group bookings (migration 033)
CREATE INDEX IF NOT EXISTS idx_bookings_group_id ON bookings(group_id);


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
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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
CREATE INDEX IF NOT EXISTS idx_booking_events_actor_type_time ON booking_events(actor_id, event_type, created_at);

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
CREATE TABLE IF NOT EXISTS payments (
    payment_id SERIAL PRIMARY KEY,
    booking_id INT UNIQUE REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    
    amount NUMERIC(10,2) NOT NULL,
    
    -- Payment gateway details
    gateway VARCHAR(50) NOT NULL, -- 'xendit', 'gcash', 'paymaya', 'cash'
    transaction_id VARCHAR(100), -- Xendit's unique transaction ID
    
    -- Payment status
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'expired', 'rejected')),
    
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
    
    -- Migration 032: Notes (for verification/rejection reasons)
    notes TEXT,
    
    -- Ensure positive amount
    CHECK (amount >= 0)
);

-- Indexes for payments
CREATE INDEX IF NOT EXISTS idx_payments_booking ON payments(booking_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_transaction_id ON payments(transaction_id);
CREATE INDEX IF NOT EXISTS idx_payments_gateway ON payments(gateway);
-- For querying Xendit responses
CREATE INDEX IF NOT EXISTS idx_payments_gateway_response ON payments USING GIN (gateway_response);
CREATE INDEX IF NOT EXISTS idx_payments_proof_url ON payments(proof_url) WHERE proof_url IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_verified ON payments(verified_at) WHERE verified_at IS NOT NULL;

-- ============================================================================
-- 5. REVIEWS & RATINGS SCHEMA
-- ============================================================================

-- Client's review of the session
CREATE TABLE IF NOT EXISTS reviews (
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
CREATE INDEX IF NOT EXISTS idx_reviews_therapist ON reviews(therapist_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_client ON reviews(client_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_booking ON reviews(booking_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_created_at ON reviews(created_at DESC);

-- Migration 018: Prevent duplicate reviews for the same booking
CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_unique_booking_non_deleted 
ON reviews(booking_id) 
WHERE deleted_at IS NULL;


-- Therapist's review of the client
CREATE TABLE IF NOT EXISTS client_reviews (
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
CREATE INDEX IF NOT EXISTS idx_client_reviews_client ON client_reviews(client_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_client_reviews_therapist ON client_reviews(therapist_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_client_reviews_booking ON client_reviews(booking_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- 6. REAL-TIME FEATURES SCHEMA
-- ============================================================================

-- Real-time location tracking (for live map)
CREATE TABLE IF NOT EXISTS live_locations (
    location_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    latitude NUMERIC(9,6) NOT NULL,
    longitude NUMERIC(9,6) NOT NULL,
    
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE (user_id) -- one active row per user
);

-- Indexes for live locations
-- CRITICAL: For nearby therapist search
CREATE INDEX IF NOT EXISTS idx_live_locations_coords ON live_locations(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_live_locations_updated ON live_locations(last_updated);

-- ============================================================================
-- 7. MESSAGING SCHEMA
-- ============================================================================

-- Chat: A thread between participants
CREATE TABLE IF NOT EXISTS conversations (
    conversation_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    is_group BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    ,updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for conversations
CREATE INDEX IF NOT EXISTS idx_conversations_booking ON conversations(booking_id);

-- Chat: Links users to a conversation
CREATE TABLE IF NOT EXISTS conversation_participants (
    conversation_id INT REFERENCES conversations(conversation_id) ON DELETE RESTRICT,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    role VARCHAR(20) DEFAULT 'member', -- 'client', 'therapist', 'admin'
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (conversation_id, user_id)
);

-- Indexes for conversation participants
CREATE INDEX IF NOT EXISTS idx_conversation_participants_user ON conversation_participants(user_id, conversation_id);

-- Chat: The actual messages
CREATE TABLE IF NOT EXISTS messages (
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
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(conversation_id) 
    WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);

-- Logs for the in-app call feature
CREATE TABLE IF NOT EXISTS call_logs (
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
CREATE INDEX IF NOT EXISTS idx_call_logs_conversation ON call_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_call_logs_initiator ON call_logs(initiator_id);

-- ============================================================================
-- 8. SAFETY & EMERGENCY SCHEMA
-- ============================================================================

-- Logs for the emergency button
CREATE TABLE IF NOT EXISTS emergency_alerts (
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
CREATE INDEX IF NOT EXISTS idx_emergency_alerts_status ON emergency_alerts(status);
CREATE INDEX IF NOT EXISTS idx_emergency_alerts_booking ON emergency_alerts(booking_id);
CREATE INDEX IF NOT EXISTS idx_emergency_alerts_triggered_by ON emergency_alerts(triggered_by);
CREATE INDEX IF NOT EXISTS idx_emergency_alerts_triggered_at ON emergency_alerts(triggered_at DESC);

-- ============================================================================
-- 9. ADMIN & AUDIT SCHEMA
-- ============================================================================

-- Audit log for admin actions
CREATE TABLE IF NOT EXISTS admin_actions (
    action_id SERIAL PRIMARY KEY,
    admin_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT, -- The user being acted upon
    
    action_type VARCHAR(50) NOT NULL, -- e.g., 'warning', 'ban', 'refund'
    details TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for admin actions
CREATE INDEX IF NOT EXISTS idx_admin_actions_admin ON admin_actions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_user ON admin_actions(user_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_type ON admin_actions(action_type);
CREATE INDEX IF NOT EXISTS idx_admin_actions_created_at ON admin_actions(created_at DESC);

-- ============================================================================
-- 10. ADDITIONAL FEATURES
-- ============================================================================

-- Notification history (was missing in original schema)
CREATE TABLE IF NOT EXISTS notifications (
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
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id, is_read) 
    WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);

-- Referral tracking (for growth features)
CREATE TABLE IF NOT EXISTS referrals (
    referral_id SERIAL PRIMARY KEY,
    referrer_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    referee_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    reward_amount NUMERIC(10,2),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure positive reward
    CHECK (reward_amount >= 0)
);

-- Indexes for referrals
CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referrals_referee ON referrals(referee_id);

-- ============================================================================
-- 10. SUPPORT & TICKETING SCHEMA
-- ============================================================================

-- Defines the support tickets submitted by users
CREATE TABLE IF NOT EXISTS support_tickets (
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
CREATE INDEX IF NOT EXISTS idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_support_tickets_booking ON support_tickets(booking_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_created_at ON support_tickets(created_at DESC);

-- Attachments for tickets (Images/Screenshots)
CREATE TABLE IF NOT EXISTS support_ticket_attachments (
    attachment_id SERIAL PRIMARY KEY,
    ticket_id INT REFERENCES support_tickets(ticket_id) ON DELETE CASCADE,
    file_url TEXT NOT NULL,
    file_type VARCHAR(50) DEFAULT 'image',
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_support_ticket_attachments_ticket ON support_ticket_attachments(ticket_id);

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
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- Apply to other tables that have `updated_at`
DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;
CREATE TRIGGER update_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_branches_updated_at ON branches;
CREATE TRIGGER update_branches_updated_at
    BEFORE UPDATE ON branches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_services_updated_at ON services;
CREATE TRIGGER update_services_updated_at
    BEFORE UPDATE ON services
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_promotions_updated_at ON promotions;
CREATE TRIGGER update_promotions_updated_at
    BEFORE UPDATE ON promotions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_therapist_profiles_updated_at ON therapist_profiles;
CREATE TRIGGER update_therapist_profiles_updated_at
    BEFORE UPDATE ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_reviews_updated_at ON reviews;
CREATE TRIGGER update_reviews_updated_at
    BEFORE UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_client_reviews_updated_at ON client_reviews;
CREATE TRIGGER update_client_reviews_updated_at
    BEFORE UPDATE ON client_reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_conversations_updated_at ON conversations;
CREATE TRIGGER update_conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_bookings_updated_at ON bookings;
CREATE TRIGGER update_bookings_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_support_tickets_updated_at ON support_tickets;
CREATE TRIGGER update_support_tickets_updated_at
    BEFORE UPDATE ON support_tickets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_booking_assignment_queue_updated_at ON booking_assignment_queue;
CREATE TRIGGER update_booking_assignment_queue_updated_at
    BEFORE UPDATE ON booking_assignment_queue
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

DROP TRIGGER IF EXISTS trg_update_therapist_rating ON reviews;
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

DROP TRIGGER IF EXISTS trg_update_client_rating ON client_reviews;
CREATE TRIGGER trg_update_client_rating
    AFTER INSERT OR UPDATE ON client_reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_client_rating();

-- ============================================================================
-- 5. RATE LIMITING SCHEMA
-- ============================================================================

-- Create rate limit tracking table for auth endpoints
CREATE TABLE IF NOT EXISTS auth_rate_limits (
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
CREATE INDEX IF NOT EXISTS idx_auth_rate_limits_identifier ON auth_rate_limits(identifier);
CREATE INDEX IF NOT EXISTS idx_auth_rate_limits_locked_until ON auth_rate_limits(locked_until);
-- ============================================================================
-- 13. FINANCE & LEDGER SCHEMA
-- ============================================================================

-- Create ENUM types for ledger entry classification
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_entry_type') THEN
        
    CREATE TYPE ledger_entry_type AS ENUM ('credit', 'debit');

    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_category') THEN
        
    CREATE TYPE ledger_category AS ENUM (
            'revenue',         -- Client payments (raw booking total)
            'commission',      -- Platform's cut (platform_fee)
            'payout',          -- Therapist earnings
            'expense',         -- Operating costs (rent, salaries, marketing)
            'refund',          -- Client refunds
            'adjustment'       -- Manual corrections
        );

    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_entry_status') THEN
        CREATE TYPE ledger_entry_status AS ENUM ('pending', 'approved', 'rejected');
    END IF;
END$$;

-- Attempt to add 'approved' and 'rejected' values if the type existed but was missing them.
-- NOTE: In some Postgres versions/drivers, this might need to be run outside of a transaction.
-- If this fails, the backfill below will fallback to 'pending'.
DO $$ 
BEGIN
    BEGIN
        ALTER TYPE ledger_entry_status ADD VALUE 'approved';
    EXCEPTION
        WHEN duplicate_object THEN null;
    END;
    BEGIN
        ALTER TYPE ledger_entry_status ADD VALUE 'rejected';
    EXCEPTION
        WHEN duplicate_object THEN null;
    END;
END $$;

-- Create ledger_entries table
CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id       BIGSERIAL PRIMARY KEY,
    booking_id     BIGINT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    entry_type     ledger_entry_type NOT NULL,
    category       ledger_category NOT NULL,
    amount         NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    description    TEXT,
    
    -- Proof & Status (from Migration 030)
    proof_url      TEXT,
    status         ledger_entry_status NOT NULL DEFAULT 'pending',
    reviewed_by    BIGINT REFERENCES users(user_id),
    reviewed_at    TIMESTAMPTZ,

    entry_date     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by     BIGINT REFERENCES users(user_id) ON DELETE SET NULL  -- For manual entries (e.g., expenses)
);

-- Indexes for common ledger queries
CREATE INDEX IF NOT EXISTS idx_ledger_entries_booking_id ON ledger_entries(booking_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_entry_date ON ledger_entries(entry_date);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_category ON ledger_entries(category);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_status ON ledger_entries(status);

-- ============================================================================
-- 14. COMPLEX BOOKINGS & PRODUCTS (Migration 033)
-- ============================================================================

-- =============================================================================
-- 1. PRODUCTS TABLE (Add-ons catalog)
-- =============================================================================
CREATE TABLE IF NOT EXISTS products (
    product_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price NUMERIC(10,2) NOT NULL DEFAULT 0,      -- Selling price (what customer pays)
    cost NUMERIC(10,2) NOT NULL DEFAULT 0,       -- Business cost (for profit margin)
    image_url TEXT,
    category VARCHAR(50) DEFAULT 'add_on', -- e.g., 'add_on', 'linen', 'wellness'
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- 2. BOOKING ADDONS (Links products to bookings)
-- =============================================================================
CREATE TABLE IF NOT EXISTS booking_addons (
    addon_id SERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES products(product_id) ON DELETE RESTRICT,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    price_at_booking NUMERIC(10,2) NOT NULL, -- Snapshot of price at time of booking
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fetching addons by booking
CREATE INDEX IF NOT EXISTS idx_booking_addons_booking_id ON booking_addons(booking_id);

-- =============================================================================
-- 3. SEED: Sample products
-- =============================================================================
INSERT INTO products (name, description, price, category) VALUES
    ('Premium Massage Oil', 'Lavender-scented premium oil', 150.00, 'add_on'),
    ('Bed Linen Set', 'Fresh linens for the session', 100.00, 'linen'),
    ('Vicks Vaporub', 'Soothing menthol rub', 80.00, 'wellness'),
    ('Extra Towel', 'Additional towel for the session', 50.00, 'linen')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- 15. SERVICE AREAS & COVERAGE (Migration 036)
-- ============================================================================

-- =============================================================================
-- 1. SERVICE AREAS (The Configuration Catalog)
-- =============================================================================
-- Stores all cities and barangays with their operational status.
-- Uses canonical area_key values that match the current service-area repository contract.

CREATE TABLE IF NOT EXISTS service_areas (
    area_id SERIAL PRIMARY KEY,
    area_key TEXT NOT NULL UNIQUE,                  -- Canonical service-area key (city or barangay)
    parent_code VARCHAR(20),                          -- NULL for cities, city_code for barangays
    name VARCHAR(150) NOT NULL,                       -- Human-readable name
    level VARCHAR(20) NOT NULL CHECK (level IN ('region', 'province', 'city', 'barangay')),
    status VARCHAR(20) NOT NULL DEFAULT 'not_supported' 
        CHECK (status IN ('covered', 'banned', 'not_supported')),
    lat NUMERIC(9,6),                                 -- Centroid latitude for distance calc
    lng NUMERIC(9,6),                                 -- Centroid longitude for distance calc
    cached_request_count INT NOT NULL DEFAULT 0,      -- Denormalized count for fast dashboard queries
    min_booking_minutes INT NOT NULL DEFAULT 0,       -- Minimum booking duration for this area
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast status lookups during booking validation
CREATE INDEX IF NOT EXISTS idx_service_areas_area_key ON service_areas(area_key);
CREATE INDEX IF NOT EXISTS idx_service_areas_status ON service_areas(status);
CREATE INDEX IF NOT EXISTS idx_service_areas_parent ON service_areas(parent_code);

-- =============================================================================
-- 2. AREA COVERAGE REQUESTS (The Interest/Demand Log)
-- =============================================================================
-- Tracks individual user requests for coverage in unsupported areas.
-- Enables re-engagement campaigns when areas launch.

CREATE TABLE IF NOT EXISTS area_coverage_requests (
    request_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    area_key TEXT NOT NULL REFERENCES service_areas(area_key),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Prevent spam: one request per user per area
    CONSTRAINT uq_user_area_request UNIQUE (user_id, area_key)
);

-- Index for counting requests per area and for user lookups
CREATE INDEX IF NOT EXISTS idx_area_requests_area_key ON area_coverage_requests(area_key);
CREATE INDEX IF NOT EXISTS idx_area_requests_user ON area_coverage_requests(user_id);

-- =============================================================================
-- 3. TRIGGER: Auto-update cached_request_count
-- =============================================================================
-- Keeps the denormalized count in sync without manual updates.

CREATE OR REPLACE FUNCTION update_area_request_count()
RETURNS TRIGGER AS $$
BEGIN
    -- Update the count for the affected area (if it exists in service_areas)
    UPDATE service_areas 
    SET cached_request_count = (
        SELECT COUNT(*) FROM area_coverage_requests WHERE area_key = COALESCE(NEW.area_key, OLD.area_key)
    ),
    updated_at = NOW()
    WHERE area_key = COALESCE(NEW.area_key, OLD.area_key);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_area_request_count_update ON area_coverage_requests;

CREATE TRIGGER trg_area_request_count_update
    AFTER INSERT OR DELETE ON area_coverage_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_area_request_count();

-- =============================================================================
-- 4. SEED: Initial Launch Cities (NCR)
-- =============================================================================
-- Populate with initial covered areas. Coordinates are approximate centroids.

INSERT INTO service_areas (area_key, name, level, status, lat, lng) VALUES
    -- NCR Cities (Covered)
    ('137600000', 'Makati', 'city', 'covered', 14.5547, 121.0244),
    ('137500000', 'Taguig', 'city', 'covered', 14.5176, 121.0509),
    ('137400000', 'Pasig', 'city', 'covered', 14.5764, 121.0851)
ON CONFLICT (area_key) DO UPDATE SET
    status = EXCLUDED.status,
    lat = EXCLUDED.lat,
    lng = EXCLUDED.lng,
    updated_at = NOW();

-- ============================================================================
-- 16. THERAPIST WALLET SYSTEM (Migration 041)
-- ============================================================================

-- =============================================================================
-- 1. THERAPIST WALLETS (Balance Tracking)
-- =============================================================================
CREATE TABLE IF NOT EXISTS therapist_wallets (
    wallet_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE RESTRICT,
    
    -- Balance tracking
    available_balance NUMERIC(12,2) NOT NULL DEFAULT 0,  -- Ready for withdrawal
    pending_balance NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Held for 24h after booking
    
    -- Lifetime totals (for dashboard/reporting)
    total_earned NUMERIC(12,2) NOT NULL DEFAULT 0,       -- Sum of all earnings
    total_withdrawn NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Sum of all payouts
    total_advances NUMERIC(12,2) NOT NULL DEFAULT 0,     -- Sum of all cash advances
    
    -- Payout settings
    minimum_payout NUMERIC(12,2) NOT NULL DEFAULT 500,   -- Minimum withdrawal amount
    last_payout_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Balance can go negative due to cash advances
    CHECK (pending_balance >= 0)
);

-- Index for fast therapist lookup
CREATE INDEX IF NOT EXISTS idx_therapist_wallets_therapist ON therapist_wallets(therapist_id);

-- =============================================================================
-- 2. WALLET TRANSACTIONS (Audit Trail)
-- =============================================================================
-- Every balance change creates a transaction record for full traceability.
CREATE TABLE IF NOT EXISTS wallet_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    ledger_entry_id BIGINT REFERENCES ledger_entries(entry_id) ON DELETE SET NULL,
    
    -- Transaction details
    type VARCHAR(30) NOT NULL CHECK (type IN (
        'earning',           -- From completed booking
        'earning_released',  -- Moved from pending to available
        'payout',            -- Withdrawal to external account
        'cash_advance',      -- Pre-payment to therapist
        'advance_repayment', -- Deducted from earnings to repay advance
        'adjustment',        -- Manual correction by admin
        'refund_clawback'    -- Returned due to client refund
    )),
    
    amount NUMERIC(12,2) NOT NULL,              -- Positive for credit, negative for debit
    balance_after NUMERIC(12,2) NOT NULL,       -- Snapshot of available_balance after txn
    pending_after NUMERIC(12,2) NOT NULL DEFAULT 0, -- Snapshot of pending_balance after txn
    
    description TEXT,
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL, -- Admin who processed (if applicable)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_wallet_txns_wallet ON wallet_transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_booking ON wallet_transactions(booking_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_type ON wallet_transactions(type);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_created ON wallet_transactions(created_at DESC);

-- =============================================================================
-- 3. PAYOUT REQUESTS (Withdrawal Queue)
-- =============================================================================
-- Tracks therapist requests to withdraw funds.
CREATE TABLE IF NOT EXISTS payout_requests (
    request_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    amount NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    payout_method VARCHAR(30) NOT NULL CHECK (payout_method IN ('gcash', 'bank_transfer', 'cash')),
    account_details JSONB, -- {account_name, account_number, bank_name} for bank; {phone} for gcash
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending',     -- Awaiting admin approval
        'approved',    -- Approved, processing payment
        'completed',   -- Payment sent
        'rejected',    -- Rejected by admin
        'cancelled'    -- Cancelled by therapist
    )),
    
    -- Processing info
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    transaction_reference TEXT, -- External payment reference (bank txn, GCash ref)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payout_requests_wallet ON payout_requests(wallet_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_therapist ON payout_requests(therapist_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_status ON payout_requests(status);

-- =============================================================================
-- 4. CASH ADVANCE RECORDS
-- =============================================================================
-- Tracks cash advances given to therapists (to be repaid from future earnings).
CREATE TABLE IF NOT EXISTS cash_advances (
    advance_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    original_amount NUMERIC(12,2) NOT NULL CHECK (original_amount > 0),
    remaining_balance NUMERIC(12,2) NOT NULL CHECK (remaining_balance >= 0),
    repayment_rate NUMERIC(5,2) NOT NULL DEFAULT 50.00, -- % of each earning to deduct (e.g., 50%)
    
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN (
        'active',    -- Being repaid from earnings
        'paid_off',  -- Fully repaid
        'written_off' -- Admin wrote off the balance
    )),
    
    -- Approval info
    approved_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    reason TEXT,
    
    paid_off_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cash_advances_wallet ON cash_advances(wallet_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_therapist ON cash_advances(therapist_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_status ON cash_advances(status) WHERE status = 'active';

-- =============================================================================
-- 5. AUTO-CREATE WALLET ON THERAPIST PROFILE CREATION
-- =============================================================================
CREATE OR REPLACE FUNCTION create_therapist_wallet()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO therapist_wallets (therapist_id)
    VALUES (NEW.therapist_id)
    ON CONFLICT (therapist_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_create_wallet_on_therapist ON therapist_profiles;

CREATE TRIGGER trg_create_wallet_on_therapist
    AFTER INSERT ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION create_therapist_wallet();

-- =============================================================================
-- 6. BACKFILL: Create wallets for existing therapists
-- =============================================================================
INSERT INTO therapist_wallets (therapist_id)
SELECT therapist_id FROM therapist_profiles
WHERE therapist_id NOT IN (SELECT therapist_id FROM therapist_wallets)
ON CONFLICT (therapist_id) DO NOTHING;

-- Backfill lifetime totals from ledger
-- Note: Only includes approved entries if status column exists (migration 030)
UPDATE therapist_wallets w
SET total_earned = COALESCE((
    SELECT SUM(amount) FROM ledger_entries le 
    WHERE le.category = 'payout' 
    AND le.entry_type = 'debit'
    AND (
        -- If status column exists, only count approved entries
        -- Otherwise, count all entries (for backwards compatibility)
        NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'ledger_entries' AND column_name = 'status'
        )
        OR le.status::text = 'approved'
    )
    AND le.booking_id IN (SELECT booking_id FROM bookings WHERE therapist_id = w.therapist_id)
), 0);

-- Set available_balance to total_earned (since no withdrawals recorded yet in new system)
UPDATE therapist_wallets
SET available_balance = total_earned
WHERE total_earned > 0;

-- ============================================================================
-- 17. RIDER MODULE (Migrations 041 + 043 + 044)
-- ============================================================================

-- =============================================================================
-- 1. RIDER PROFILES
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_profiles (
    rider_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    vehicle_type VARCHAR(50), -- 'motorcycle', 'car', 'suv'
    license_plate VARCHAR(20),
    license_number VARCHAR(50),
    is_online BOOLEAN DEFAULT FALSE,
    current_location GEOGRAPHY(POINT, 4326), -- SOTA: geography for accurate distances
    last_location_update TIMESTAMPTZ,
    rating DECIMAL(3,2) DEFAULT 5.0,
    total_trips INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- GiST Index for fast geospatial lookups
CREATE INDEX IF NOT EXISTS idx_rider_location ON rider_profiles USING GIST(current_location);
CREATE INDEX IF NOT EXISTS idx_rider_online ON rider_profiles(is_online) WHERE is_online = true;

-- =============================================================================
-- 2. RIDES TABLE  
-- =============================================================================
CREATE TABLE IF NOT EXISTS rides (
    ride_id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT REFERENCES rider_profiles(rider_id),
    passenger_id BIGINT NOT NULL REFERENCES users(user_id),
    booking_id BIGINT REFERENCES bookings(booking_id),
    
    pickup_lat DECIMAL(10, 7) NOT NULL,
    pickup_long DECIMAL(10, 7) NOT NULL,
    pickup_address TEXT,
    
    dropoff_lat DECIMAL(10, 7) NOT NULL,
    dropoff_long DECIMAL(10, 7) NOT NULL,
    dropoff_address TEXT,
    
    distance_km DECIMAL(6,2),
    pricing_snapshot JSONB,
    
    status VARCHAR(30) DEFAULT 'pending',
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    offered_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    arrived_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancellation_reason TEXT,
    
    retry_count INT NOT NULL DEFAULT 0,
    last_retried_at TIMESTAMPTZ,
    scheduled_for TIMESTAMPTZ,
    
    -- Migration 043: Ride type
    ride_type VARCHAR(20) DEFAULT 'outbound' CHECK (ride_type IN ('outbound', 'return')),
    
    -- Migration 044: Rider earnings
    rider_earnings_cents INT,
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rides_rider ON rides(rider_id);
CREATE INDEX IF NOT EXISTS idx_rides_passenger ON rides(passenger_id);
CREATE INDEX IF NOT EXISTS idx_rides_status ON rides(status);
CREATE INDEX IF NOT EXISTS idx_rides_booking_id ON rides(booking_id) WHERE booking_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_rides_type_status ON rides(ride_type, status);

-- Composite index for GetUnmatchedRidesForRetry
CREATE INDEX IF NOT EXISTS idx_rides_retry_lookup ON rides (status, rider_id, retry_count, last_retried_at)
  WHERE status = 'pending' AND rider_id IS NULL;

-- Partial index for schedule-aware rider filtering
CREATE INDEX IF NOT EXISTS idx_rides_active_schedule ON rides (rider_id, scheduled_for)
  WHERE status IN ('accepted', 'arrived_pickup', 'in_progress', 'arrived_dropoff') AND scheduled_for IS NOT NULL;

-- =============================================================================
-- RIDE OFFERS (Migration 059)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ride_offers (
    offer_id BIGSERIAL PRIMARY KEY,
    ride_id BIGINT NOT NULL REFERENCES rides(ride_id) ON DELETE CASCADE,
    rider_id BIGINT NOT NULL REFERENCES rider_profiles(rider_id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    UNIQUE(ride_id, rider_id)
);

CREATE INDEX IF NOT EXISTS idx_ride_offers_ride_id ON ride_offers(ride_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_rider_id ON ride_offers(rider_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_status ON ride_offers(status);
CREATE INDEX IF NOT EXISTS idx_ride_offers_expires_at ON ride_offers(expires_at) WHERE status = 'pending';

-- =============================================================================
-- 3. RIDE PRICING CONFIGURATION
-- =============================================================================
CREATE TABLE IF NOT EXISTS ride_pricing_config (
    config_id SERIAL PRIMARY KEY,
    config_key VARCHAR(50) UNIQUE DEFAULT 'default',
    base_distance_km DECIMAL(4,2) DEFAULT 3.0,
    base_rate DECIMAL(8,2) DEFAULT 50.0,
    per_km_rate DECIMAL(8,2) DEFAULT 10.0,
    per_100m_rate DECIMAL(8,2) DEFAULT 1.0,
    min_fare DECIMAL(8,2) DEFAULT 50.0,
    max_fare DECIMAL(8,2) DEFAULT 150.0,
    surge_enabled BOOLEAN DEFAULT FALSE,
    dispatch_buffer_minutes INTEGER DEFAULT 30,
    default_vehicle_type VARCHAR(50) DEFAULT 'motorcycle',
    surge_multiplier DECIMAL(3,2) DEFAULT 1.0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO ride_pricing_config (config_key) VALUES ('default') ON CONFLICT DO NOTHING;

-- =============================================================================
-- 4. RIDER WALLETS (Migration 044)
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_wallets (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    balance_cents INT NOT NULL DEFAULT 0,
    total_earned_cents INT NOT NULL DEFAULT 0,
    total_withdrawn_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_wallet_balance_non_negative CHECK (balance_cents >= 0),
    CONSTRAINT rider_wallet_totals_consistent CHECK (total_earned_cents >= total_withdrawn_cents)
);

-- =============================================================================
-- RIDER PAYOUT METHODS (Migration 058)
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_payout_methods (
    id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method_type VARCHAR(20) NOT NULL CHECK (method_type IN ('bank', 'gcash', 'paymaya', 'grabpay')),
    provider_name VARCHAR(100) NOT NULL,
    account_number VARCHAR(100) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rider_payout_methods_rider ON rider_payout_methods(rider_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rider_payout_methods_default ON rider_payout_methods(rider_id) WHERE is_default = TRUE;

-- =============================================================================
-- 5. RIDER TRANSACTIONS (Migration 044)
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_transactions (
    transaction_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('ride_earning', 'payout', 'adjustment', 'bonus')),
    amount_cents INT NOT NULL,
    ride_id INT REFERENCES rides(ride_id) ON DELETE SET NULL,
    payout_method_id INT REFERENCES rider_payout_methods(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    
    CONSTRAINT rider_transaction_amount_non_zero CHECK (amount_cents != 0)
);

CREATE INDEX IF NOT EXISTS idx_rider_transactions_rider ON rider_transactions(rider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_ride ON rider_transactions(ride_id);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_status ON rider_transactions(status) WHERE status = 'pending';

-- =============================================================================
-- 6. RIDER PERFORMANCE METRICS (Migration 044)
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_performance_metrics (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    total_offers_received INT NOT NULL DEFAULT 0,
    total_rides_accepted INT NOT NULL DEFAULT 0,
    total_rides_completed INT NOT NULL DEFAULT 0,
    total_rides_cancelled INT NOT NULL DEFAULT 0,
    acceptance_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    completion_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    average_rating DECIMAL(3,2) DEFAULT NULL,
    total_ratings INT NOT NULL DEFAULT 0,
    rating_sum INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_acceptance_rate_valid CHECK (acceptance_rate >= 0 AND acceptance_rate <= 100),
    CONSTRAINT rider_completion_rate_valid CHECK (completion_rate >= 0 AND completion_rate <= 100),
    CONSTRAINT rider_average_rating_valid CHECK (average_rating IS NULL OR (average_rating >= 1 AND average_rating <= 5))
);

-- =============================================================================
-- 7. RIDER EMERGENCY CONTACTS (Migration 044)
-- =============================================================================
CREATE TABLE IF NOT EXISTS rider_emergency_contacts (
    contact_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    full_name VARCHAR(100) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    relationship VARCHAR(50),
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rider_emergency_contacts_rider ON rider_emergency_contacts(rider_id);

-- =============================================================================
-- 8. MIGRATION 043: Add home_address_id to therapist_profiles
-- =============================================================================
ALTER TABLE therapist_profiles
ADD COLUMN IF NOT EXISTS home_address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_therapist_profiles_home_address 
    ON therapist_profiles(home_address_id) WHERE home_address_id IS NOT NULL;

-- =============================================================================
-- 9. TRIGGERS: Auto-update rider wallet on ride completion
-- =============================================================================
CREATE OR REPLACE FUNCTION update_rider_wallet_on_earning()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' AND NEW.rider_earnings_cents IS NOT NULL AND NEW.rider_id IS NOT NULL THEN
        -- Upsert rider wallet
        INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents)
        SELECT u.user_id, NEW.rider_earnings_cents, NEW.rider_earnings_cents
        FROM rider_profiles rp
        JOIN users u ON u.user_id = rp.user_id
        WHERE rp.rider_id = NEW.rider_id
        ON CONFLICT (rider_id) DO UPDATE SET
            balance_cents = rider_wallets.balance_cents + NEW.rider_earnings_cents,
            total_earned_cents = rider_wallets.total_earned_cents + NEW.rider_earnings_cents,
            updated_at = NOW();
        
        -- Create transaction record
        INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, ride_id, status, description)
        SELECT u.user_id, 'ride_earning', NEW.rider_earnings_cents, NEW.ride_id, 'completed',
               'Earnings from ride #' || NEW.ride_id
        FROM rider_profiles rp
        JOIN users u ON u.user_id = rp.user_id
        WHERE rp.rider_id = NEW.rider_id;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_rider_wallet ON rides;

CREATE TRIGGER trg_update_rider_wallet
    AFTER UPDATE ON rides
    FOR EACH ROW
    WHEN (NEW.status = 'completed' AND NEW.rider_earnings_cents IS NOT NULL)
    EXECUTE FUNCTION update_rider_wallet_on_earning();

-- ============================================================================
-- 18. SHOPPING CART SYSTEM (Migration 035)
-- ============================================================================

CREATE TABLE IF NOT EXISTS carts (
    cart_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_carts_user_id ON carts(user_id);

CREATE TABLE IF NOT EXISTS cart_items (
    cart_item_id SERIAL PRIMARY KEY,
    cart_id INT NOT NULL REFERENCES carts(cart_id) ON DELETE CASCADE,
    service_id INT NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    
    guest_name VARCHAR(100) NOT NULL DEFAULT 'Self',
    duration_minutes INT NOT NULL DEFAULT 60,
    gender_preference VARCHAR(10) CHECK (gender_preference IN ('male', 'female', 'any')) DEFAULT 'any',
    pressure_preference VARCHAR(10) CHECK (pressure_preference IN ('soft', 'medium', 'hard')) DEFAULT 'medium',
    notes TEXT,
    
    sequence_number INT NOT NULL DEFAULT 0,
    start_condition VARCHAR(20) CHECK (start_condition IN ('fixed_time', 'after_previous')) DEFAULT 'fixed_time',
    
    addons JSONB DEFAULT '[]'::jsonb,
    
    date_added TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (duration_minutes > 0)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON cart_items(cart_id);

CREATE OR REPLACE FUNCTION update_cart_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE carts SET updated_at = NOW() WHERE cart_id = NEW.cart_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_cart_items_update_cart_timestamp ON cart_items;

CREATE TRIGGER trg_cart_items_update_cart_timestamp
    AFTER INSERT OR UPDATE OR DELETE ON cart_items
    FOR EACH ROW
    EXECUTE FUNCTION update_cart_timestamp();

-- ============================================================================
-- 19. SCHEMA FIXES (Migration 034)
-- ============================================================================

-- Fix branches table
ALTER TABLE branches ADD COLUMN IF NOT EXISTS branch_name VARCHAR(150);
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'name'
  ) THEN
    UPDATE branches SET branch_name = name WHERE branch_name IS NULL AND name IS NOT NULL;
  END IF;
END $$;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS address_line VARCHAR(255);
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'address'
  ) THEN
    UPDATE branches SET address_line = address WHERE address_line IS NULL AND address IS NOT NULL;
  END IF;
END $$;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS barangay VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS province VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS latitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS contact_no VARCHAR(20);
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'phone'
  ) THEN
    UPDATE branches SET contact_no = phone WHERE contact_no IS NULL AND phone IS NOT NULL;
  END IF;
END $$;

-- Fix admin_actions table
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_type VARCHAR(50);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_id INT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS old_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS new_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS performed_at TIMESTAMP;
UPDATE admin_actions SET performed_at = created_at WHERE performed_at IS NULL;

-- Fix referrals table
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referred_id INT;
UPDATE referrals SET referred_id = referee_id WHERE referred_id IS NULL AND referee_id IS NOT NULL;

ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referral_code VARCHAR(50);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS status VARCHAR(20);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS reward_earned BOOLEAN DEFAULT FALSE;
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;

-- Referral rewards table
CREATE TABLE IF NOT EXISTS referral_rewards (
    reward_id SERIAL PRIMARY KEY,
    referral_id INT REFERENCES referrals(referral_id),
    user_id INT REFERENCES users(user_id),
    reward_type VARCHAR(50),
    reward_amount NUMERIC(10,2),
    status VARCHAR(20) DEFAULT 'pending',
    expires_at TIMESTAMP,
    redeemed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

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
        EXECUTE 'ALTER TABLE messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMP';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'messages' AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP';
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



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;


-- 2) Normalize booking_events confirm_start event types (from migration 007)
BEGIN;

UPDATE booking_events
SET event_type = 'confirm_start'
WHERE event_type IN ('client_confirm_start', 'therapist_confirm_start', 'admin_confirm_start');



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Migration 009: payment_method is now included in the CREATE TABLE IF NOT EXISTS statement above
-- This section is kept for backward compatibility with existing databases
-- but is a no-op for fresh databases
BEGIN;

ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20)
        CHECK (payment_method IN ('cash', 'gcash'))
        NOT NULL DEFAULT 'cash';



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;


-- 3) Backfill existing completed bookings into the ledger (from Migration 029)
-- This creates historical ledger entries for bookings completed before this schema was applied
-- if they exist in the dump but not in the ledger.
BEGIN;

-- Commission Backfill
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at, status)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'commission'::ledger_category,
    COALESCE(b.platform_fee, 0),
    'Platform commission (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW(),
    CASE 
        WHEN EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'approved' AND enumtypid = 'ledger_entry_status'::regtype) 
        THEN 'approved'::ledger_entry_status 
        ELSE 'pending'::ledger_entry_status 
    END as status
FROM bookings b
WHERE b.status = 'completed'
  AND b.platform_fee IS NOT NULL
  AND b.platform_fee > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'commission'
  );

-- Revenue Backfill (Optional)
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at, status)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'revenue'::ledger_category,
    COALESCE(b.final_total, 0),
    'Client payment (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW(),
    CASE 
        WHEN EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'approved' AND enumtypid = 'ledger_entry_status'::regtype) 
        THEN 'approved'::ledger_entry_status 
        ELSE 'pending'::ledger_entry_status 
    END as status
FROM bookings b
WHERE b.status = 'completed'
  AND b.final_total IS NOT NULL
  AND b.final_total > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'revenue'
  );

-- Payout Backfill (Optional)
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at, status)
SELECT
    b.booking_id,
    'debit'::ledger_entry_type,
    'payout'::ledger_category,
    COALESCE(b.therapist_earnings, 0),
    'Therapist payout (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW(),
    CASE 
        WHEN EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'approved' AND enumtypid = 'ledger_entry_status'::regtype) 
        THEN 'approved'::ledger_entry_status 
        ELSE 'pending'::ledger_entry_status 
    END as status
FROM bookings b
WHERE b.status = 'completed'
  AND b.therapist_earnings IS NOT NULL
  AND b.therapist_earnings > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'payout'
  );



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;


-- Migration: add therapist_service_pressures table
-- REMOVED: This table was deprecated in favor of boolean flags on therapist_services
-- (supports_soft, supports_moderate, supports_hard)
-- The table is created and immediately dropped by migration 010, so we skip it here.
-- Add void columns to ledger_entries table
ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS voided BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS voided_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS voided_reason TEXT;

-- Index for filtering voided entries
CREATE INDEX IF NOT EXISTS idx_ledger_entries_voided ON ledger_entries(voided);
-- ============================================================================
-- MIGRATION 002: Align Database Schema with API Expectations
-- ============================================================================
-- This migration updates the schema to match the API handlers and documentation
-- Run this after 001.sql has been applied

-- ============================================================================
-- 1. UPDATE ADDRESSES TABLE
-- ============================================================================

-- Rename street to street_address
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'addresses' AND column_name = 'street') THEN
        ALTER TABLE addresses RENAME COLUMN street TO street_address;
    END IF;
END $$;

-- Add missing columns
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS barangay VARCHAR(100);
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS landmark VARCHAR(200);
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 2. UPDATE BRANCHES TABLE
-- ============================================================================

-- Drop the foreign key to address_id if it exists
ALTER TABLE branches DROP COLUMN IF EXISTS address_id CASCADE;

-- Add simple address string field
ALTER TABLE branches ADD COLUMN IF NOT EXISTS address VARCHAR(255);

-- Rename phone_number to phone
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'branches' AND column_name = 'phone_number') THEN
        ALTER TABLE branches RENAME COLUMN phone_number TO phone;
    END IF;
END $$;

-- Add missing columns
ALTER TABLE branches ADD COLUMN IF NOT EXISTS operating_hours JSONB;
ALTER TABLE branches ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 3. UPDATE SERVICES TABLE
-- ============================================================================

-- Rename min_duration_minutes to duration_minutes
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'services' AND column_name = 'min_duration_minutes') THEN
        ALTER TABLE services RENAME COLUMN min_duration_minutes TO duration_minutes;
    END IF;
END $$;

-- Add missing columns
ALTER TABLE services ADD COLUMN IF NOT EXISTS category VARCHAR(50);
ALTER TABLE services ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
ALTER TABLE services ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
-- `preview_image_url` moved to initial schema in 001.sql; no-op here

-- ============================================================================
-- 4. UPDATE PROMOTIONS TABLE
-- ============================================================================

-- Rename discount_percent to discount_percentage
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'promotions' AND column_name = 'discount_percent') THEN
        ALTER TABLE promotions RENAME COLUMN discount_percent TO discount_percentage;
    END IF;
END $$;

-- Rename usage_limit to max_uses
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'promotions' AND column_name = 'usage_limit') THEN
        ALTER TABLE promotions RENAME COLUMN usage_limit TO max_uses;
    END IF;
END $$;

-- Add missing columns
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(10,2);
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS current_uses INT DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 5. UPDATE NOTIFICATIONS TABLE
-- ============================================================================

-- Add missing column
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 6. UPDATE EMERGENCY_ALERTS TABLE
-- ============================================================================

-- Add missing columns
ALTER TABLE emergency_alerts ADD COLUMN IF NOT EXISTS alert_type VARCHAR(50);
ALTER TABLE emergency_alerts ADD COLUMN IF NOT EXISTS description TEXT;

-- ============================================================================
-- 7. UPDATE THERAPIST_PROFILES TABLE
-- ============================================================================

-- Add updated_at if missing
ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 8. CREATE TRIGGERS FOR UPDATED_AT AUTO-UPDATE
-- ============================================================================

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply triggers to tables with updated_at
DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;
DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;
CREATE TRIGGER update_addresses_updated_at 
    BEFORE UPDATE ON addresses 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_branches_updated_at ON branches;
DROP TRIGGER IF EXISTS update_branches_updated_at ON branches;
CREATE TRIGGER update_branches_updated_at 
    BEFORE UPDATE ON branches 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_services_updated_at ON services;
DROP TRIGGER IF EXISTS update_services_updated_at ON services;
CREATE TRIGGER update_services_updated_at 
    BEFORE UPDATE ON services 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_promotions_updated_at ON promotions;
DROP TRIGGER IF EXISTS update_promotions_updated_at ON promotions;
CREATE TRIGGER update_promotions_updated_at 
    BEFORE UPDATE ON promotions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
CREATE TRIGGER update_notifications_updated_at 
    BEFORE UPDATE ON notifications 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_therapist_profiles_updated_at ON therapist_profiles;
DROP TRIGGER IF EXISTS update_therapist_profiles_updated_at ON therapist_profiles;
CREATE TRIGGER update_therapist_profiles_updated_at 
    BEFORE UPDATE ON therapist_profiles 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- MIGRATION COMPLETE
-- ============================================================================
-- Schema is now aligned with API expectations
-- All tables have consistent naming and required fields
-- 002_migrate_messages.sql
-- Safe, idempotent migration to align the existing `messages` table
-- with the updated application schema (content, sent_at, read_at, deleted_at).
-- This script performs conditional changes so it can be applied against
-- development databases without failing if some columns already exist.

BEGIN;

-- Rename legacy text/timestamp columns if present and add missing columns
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'message_text'
  ) THEN
    EXECUTE 'ALTER TABLE messages RENAME COLUMN message_text TO content';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'created_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages RENAME COLUMN created_at TO sent_at';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'read_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMP';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'deleted_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP';
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

-- Recreate / normalize indexes used by application
DROP INDEX IF EXISTS idx_messages_conversation;
DROP INDEX IF EXISTS idx_messages_unread;
DROP INDEX IF EXISTS idx_messages_sender;

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(conversation_id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Notes:
-- 1) If your DB is in production, review and test these changes in a staging
--    environment first. Back up the `messages` table before running.
-- 2) After applying, restart the server so repository queries/scan map to
--    the new columns (`content`, `sent_at`, `read_at`, `deleted_at`).
-- 003_add_updated_at_conversations.sql
-- Idempotent migration: add `updated_at` to `conversations` and a trigger

BEGIN;

-- Add updated_at column if missing
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Trigger function to update updated_at on row changes
CREATE OR REPLACE FUNCTION rh_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger only if it doesn't already exist
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'trg_conversations_updated_at'
  ) THEN
    EXECUTE 'DROP TRIGGER IF EXISTS trg_conversations_updated_at ON conversations;
CREATE TRIGGER trg_conversations_updated_at
      BEFORE UPDATE ON conversations
      FOR EACH ROW
      EXECUTE FUNCTION rh_set_updated_at()';
  END IF;
END;
$$;



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Notes:
-- Run this migration in your DB environment (psql or migration tool).
-- It's safe to re-run because of IF NOT EXISTS / CREATE OR REPLACE.
-- 004: Make therapist_id nullable and add booking assignment queue

ALTER TABLE bookings ALTER COLUMN therapist_id DROP NOT NULL;

-- Create a durable queue table for booking assignments
CREATE TABLE IF NOT EXISTS booking_assignment_queue (
    queue_id SERIAL PRIMARY KEY,
    booking_id INT UNIQUE REFERENCES bookings(booking_id) ON DELETE CASCADE,
    enqueued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    attempts INT DEFAULT 0,
    last_attempt_at TIMESTAMP,
    next_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_booking_assignment_queue_enqueued_at ON booking_assignment_queue(enqueued_at);
-- Migration: add accept_assignments toggle to therapists
-- Adds a boolean column `accept_assignments` defaulting to true

-- The project stores therapist rows in `therapist_profiles`.
ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS accept_assignments BOOLEAN NOT NULL DEFAULT TRUE;

-- Optional index if you need to query by this flag frequently
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_accept_assignments ON therapist_profiles(accept_assignments);
-- ============================================================================
-- Migration: 006 - Booking events + assignment/arrival/cancellation timestamps
-- ============================================================================
-- Adds audit events for bookings and a few helpful timestamp/cancellation columns
-- This migration is idempotent (uses IF NOT EXISTS checks)

BEGIN;

-- Add new timestamp and cancellation columns to bookings
ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS therapist_arrived_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS no_show_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS cancelled_by VARCHAR(20),
    ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS cancellation_reason TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create booking_events table to record timeline and important actions
CREATE TABLE IF NOT EXISTS booking_events (
    event_id SERIAL PRIMARY KEY,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL, -- e.g. 'created','assigned','payment_succeeded','therapist_arrived',etc.
    actor_id INT REFERENCES users(user_id) ON DELETE SET NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for booking_events to support timeline queries
CREATE INDEX IF NOT EXISTS idx_booking_events_booking ON booking_events(booking_id);
CREATE INDEX IF NOT EXISTS idx_booking_events_type ON booking_events(event_type);
CREATE INDEX IF NOT EXISTS idx_booking_events_created_at ON booking_events(created_at DESC);

-- Add updated_at trigger for bookings if not already present
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_bookings_updated_at'
    ) THEN
        DROP TRIGGER IF EXISTS update_bookings_updated_at ON bookings;
CREATE TRIGGER update_bookings_updated_at
            BEFORE UPDATE ON bookings
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END;
$$ LANGUAGE plpgsql;



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Helpful comments
COMMENT ON TABLE booking_events IS 'Timeline of booking-related events used for UI timelines, auditing and idempotency.';
-- Migration 006: add is_verified to therapist_profiles
ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;

-- Ensure non-null values
UPDATE therapist_profiles SET is_verified = FALSE WHERE is_verified IS NULL;

-- Optional index for queries filtering by verified status
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_verified ON therapist_profiles(is_verified) WHERE deleted_at IS NULL;

-- end
-- ============================================================================
-- Migration: 007 - Normalize start confirmation event types -> confirm_start
-- ============================================================================
-- This migration consolidates previous role-specific start confirmation
-- event types (client_confirm_start, therapist_confirm_start,
-- admin_confirm_start) into a single canonical `confirm_start` value.
--
-- Run this on your Supabase/Postgres instance (via psql, supabase SQL editor,
-- or migrate tool). It's safe to run multiple times (idempotent update).

BEGIN;

-- Convert any legacy role-specific confirm events into the unified type
UPDATE booking_events
SET event_type = 'confirm_start'
WHERE event_type IN ('client_confirm_start', 'therapist_confirm_start', 'admin_confirm_start');



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Optional: verify changes (run as a separate query if you want a count)
-- SELECT event_type, count(*) FROM booking_events WHERE event_type LIKE '%confirm%' GROUP BY 1 ORDER BY 2 DESC;
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

-- Migration 009: Support columns already defined in base therapist_services table (lines 176-183)
-- Backfill from therapist_service_pressures removed - table was deprecated in migration 010



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- NOTE: Do NOT DROP therapist_service_pressures in this migration. After verification, you may remove it in a later migration:
-- DROP TABLE IF EXISTS therapist_service_pressures;
-- Migration: 009 - Expand bookings.status CHECK to include application statuses
-- Idempotent: drops existing constraint if present and recreates it.

BEGIN;

-- Drop old constraint if it exists (safe to run on upgraded DBs)
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;

-- Recreate constraint with the full set of statuses used by the application
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (
  status IN (
    'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed',
    'cancelled', 'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
  )
);



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;

-- Notes:
-- 1) This migration is intentionally minimal and idempotent.
-- 2) For existing production DBs, run this migration with normal tooling or via psql.
-- 3) If you use a migration runner, ensure this file is applied after existing migrations.
-- Migration 010: drop therapist_service_pressures after migration to boolean columns

-- WARNING: Ensure migration 009 has been applied and data verified before running this.
-- This migration removes the legacy pressures table and related indexes.

BEGIN;

DROP INDEX IF EXISTS idx_tsp_service;
DROP INDEX IF EXISTS idx_tsp_therapist;

DROP TABLE IF EXISTS therapist_service_pressures;



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;
-- Migration: 010 - Update booking status from 'en_route' to 'on_the_way'
-- Description: Renames the internal status label and updates the check constraint.

BEGIN;

-- 1. Drop the old constraint (from 001.sql or 009.sql)
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;

-- 2. Update existing data to the new status labels
UPDATE bookings SET status = 'on_the_way' WHERE status = 'en_route';
UPDATE bookings SET status = 'pending' WHERE status = 'confirmed'; -- Map confirmed to pending

-- 3. Add the new constraint with 'on_the_way' and WITHOUT 'confirmed'
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (
  status IN (
    'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed',
    'cancelled', 'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
  )
);



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;
-- Merge is_available into accept_assignments and drop is_available
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'therapist_profiles' AND column_name = 'is_available') THEN
        UPDATE therapist_profiles SET accept_assignments = (accept_assignments AND is_available);
        ALTER TABLE therapist_profiles DROP COLUMN IF EXISTS is_available;
    END IF;
END $$;
-- Add reference_code to bookings table
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS reference_code VARCHAR(20);

-- Create unique index (implicitly indexes for lookup)
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_reference_code ON bookings(reference_code);

-- Optional: If we want to backfill existing bookings with a placeholder or generate them
-- For now, we leave them null or let the app handle it.
-- Code format is 'RH-YYYYMMDD-HEX', so we can't easily generate valid ones in SQL without a complex function.
-- Seed test promotions
INSERT INTO promotions (code, description, discount_amount, discount_percentage, valid_from, valid_until, max_uses, current_uses, days_of_week)
VALUES 
('WELCOME50', 'Get 50 PHP off your booking', 50.00, NULL, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 year', 1000, 0, NULL),
('SUMMER10', 'Get 10% off your booking', NULL, 10, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 year', 1000, 0, NULL)
ON CONFLICT (code) DO NOTHING;
-- Add FCM token column to users table for push notifications
ALTER TABLE users ADD COLUMN IF NOT EXISTS fcm_token TEXT;

-- CREATE INDEX IF NOT EXISTS for faster lookups when sending push notifications
CREATE INDEX IF NOT EXISTS idx_users_fcm_token ON users(fcm_token) WHERE fcm_token IS NOT NULL;
-- Migration: Add pause tracking fields to bookings table
-- Allows tracking pause/resume cycles during sessions

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS total_paused_seconds INT DEFAULT 0;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS current_pause_start TIMESTAMPTZ;

-- Down migration (for rollback):
-- ALTER TABLE bookings DROP COLUMN IF EXISTS total_paused_seconds;
-- ALTER TABLE bookings DROP COLUMN IF EXISTS current_pause_start;
BEGIN;

-- Defines the support tickets submitted by users
CREATE TABLE IF NOT EXISTS support_tickets (
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
CREATE INDEX IF NOT EXISTS idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_support_tickets_booking ON support_tickets(booking_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_created_at ON support_tickets(created_at DESC);

-- Attachments for tickets (Images/Screenshots)
CREATE TABLE IF NOT EXISTS support_ticket_attachments (
    attachment_id SERIAL PRIMARY KEY,
    ticket_id INT REFERENCES support_tickets(ticket_id) ON DELETE CASCADE,
    file_url TEXT NOT NULL,
    file_type VARCHAR(50) DEFAULT 'image',
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_support_ticket_attachments_ticket ON support_ticket_attachments(ticket_id);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_support_tickets_updated_at ON support_tickets;
CREATE TRIGGER update_support_tickets_updated_at
    BEFORE UPDATE ON support_tickets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();



-- ============================================
-- ROW LEVEL SECURITY (SOTA)
-- ============================================

COMMIT;
CREATE TABLE IF NOT EXISTS favorite_therapists (
    user_id BIGINT NOT NULL,
    therapist_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, therapist_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (therapist_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_favorite_therapists_user_id ON favorite_therapists(user_id);
-- Migration 018: Prevent duplicate reviews for the same booking
CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_unique_booking_non_deleted 
ON reviews(booking_id) 
WHERE deleted_at IS NULL;
-- Add account_status column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS account_status VARCHAR(20) NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_vip BOOLEAN NOT NULL DEFAULT FALSE;

-- Add check constraint for valid statuses
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_account_status;
ALTER TABLE users ADD CONSTRAINT check_account_status CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive', 'blocked'));

-- Add index for account_status
CREATE INDEX IF NOT EXISTS idx_users_account_status ON users(account_status);
CREATE INDEX IF NOT EXISTS idx_users_is_vip ON users(is_vip) WHERE is_vip = TRUE;
-- Migration 020: Add updated_at column to reviews table
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create trigger to automatically update updated_at if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_reviews_updated_at') THEN
        DROP TRIGGER IF EXISTS update_reviews_updated_at ON reviews;
CREATE TRIGGER update_reviews_updated_at
            BEFORE UPDATE ON reviews
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;
-- Drop the existing constraint
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_payment_method_check;

-- Add the new constraint with updated values
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_payment_method_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_payment_method_check 
    CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer'));
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
-- Migration: Payment proof tracking and extension wait time
-- 1. Add proof_url to payments table for tracking uploaded payment proofs
-- 2. Add extension_wait_seconds to bookings for accurate time tracking during approval waits

-- Add proof_url column to payments table
ALTER TABLE payments ADD COLUMN IF NOT EXISTS proof_url TEXT;

-- Add extension_wait_seconds to bookings table
-- This tracks time spent waiting for extension approval (separate from pause time)
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS extension_wait_seconds INT DEFAULT 0;

-- Add 'cancelled' status to extension_requests if not already present
ALTER TABLE booking_extension_requests DROP CONSTRAINT IF EXISTS booking_extension_requests_status_check;
ALTER TABLE booking_extension_requests ADD CONSTRAINT booking_extension_requests_status_check 
    CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled'));

-- Index for payment proof lookups
CREATE INDEX IF NOT EXISTS idx_payments_proof_url ON payments(proof_url) WHERE proof_url IS NOT NULL;
-- Migration: Add verified_by column to payments table for tracking who verified the payment
-- This complements the existing proof_url added in 023

ALTER TABLE payments ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS verified_by INT REFERENCES users(user_id) ON DELETE SET NULL;

-- Index for finding verified payments
CREATE INDEX IF NOT EXISTS idx_payments_verified ON payments(verified_at) WHERE verified_at IS NOT NULL;
-- Migration: Add payment_breakdown JSONB column to bookings table
-- This stores itemized pricing: base_price, duration_markup, extensions_total, service_snapshot_name

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS payment_breakdown JSONB;

-- Add a comment for documentation
COMMENT ON COLUMN bookings.payment_breakdown IS 'Stores itemized price breakdown: base_price, duration_markup, extensions_total, service_snapshot_name';

-- Migration: Add created_at and updated_at to payments table
-- This fixes the error where the server expects these columns but they don't exist.

ALTER TABLE payments ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Add trigger to automatically update updated_at
DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;
DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;
CREATE TRIGGER update_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- Migration: Add index for efficient therapist unassignment tracking
-- Used to query therapist_unassigned events by actor_id and event_type for daily/weekly limits

CREATE INDEX IF NOT EXISTS idx_booking_events_actor_type_time 
ON booking_events(actor_id, event_type, created_at);

-- Comment: This index supports the unassignment policy (3/day, 5/week limits)
-- by enabling efficient COUNT queries filtering on actor_id, event_type, and created_at range.
-- Migration: Add commission tracking columns
-- 1. Add therapist_commission to services table
-- 2. Add therapist_earnings and platform_fee to bookings table

-- Services: Fixed commission amount per service
ALTER TABLE services ADD COLUMN IF NOT EXISTS therapist_commission NUMERIC(10,2);

-- Comment for documentation
COMMENT ON COLUMN services.therapist_commission IS 'The fixed amount the therapist earns for the base duration of this service';

-- Bookings: Snapshot of the financial split at completion
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS therapist_earnings NUMERIC(10,2);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS platform_fee NUMERIC(10,2);

COMMENT ON COLUMN bookings.therapist_earnings IS 'Final amount payable to therapist (base + extensions share)';
COMMENT ON COLUMN bookings.platform_fee IS 'Amount retained by platform (final_total - therapist_earnings)';
-- Migration: Add ledger_entries table for Double-Entry Accounting
-- This provides a unified financial journal for revenue, payouts, and expenses.

-- Create ENUM types for ledger entry classification
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_entry_type') THEN
        
    CREATE TYPE ledger_entry_type AS ENUM ('credit', 'debit');

    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ledger_category') THEN
        
    CREATE TYPE ledger_category AS ENUM (
            'revenue',         -- Client payments (raw booking total)
            'commission',      -- Platform's cut (platform_fee)
            'payout',          -- Therapist earnings
            'expense',         -- Operating costs (rent, salaries, marketing)
            'refund',          -- Client refunds
            'adjustment'       -- Manual corrections
        );

    END IF;
END$$;

-- Create ledger_entries table
CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id       BIGSERIAL PRIMARY KEY,
    booking_id     BIGINT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    entry_type     ledger_entry_type NOT NULL,
    category       ledger_category NOT NULL,
    amount         NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    description    TEXT,
    entry_date     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by     BIGINT REFERENCES users(user_id) ON DELETE SET NULL  -- For manual entries (e.g., expenses)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_ledger_entries_booking_id ON ledger_entries(booking_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_entry_date ON ledger_entries(entry_date);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_category ON ledger_entries(category);

-- Backfill existing completed bookings into the ledger
-- This creates historical ledger entries for bookings that were completed before this migration.
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'commission'::ledger_category,
    COALESCE(b.platform_fee, 0),
    'Platform commission (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.platform_fee IS NOT NULL
  AND b.platform_fee > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'commission'
  );

-- Optional: Also backfill revenue (client payment) entries for full double-entry
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'credit'::ledger_entry_type,
    'revenue'::ledger_category,
    COALESCE(b.final_total, 0),
    'Client payment (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.final_total IS NOT NULL
  AND b.final_total > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'revenue'
  );

-- Optional: Also backfill payout (therapist earnings) entries
INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_at)
SELECT
    b.booking_id,
    'debit'::ledger_entry_type,
    'payout'::ledger_category,
    COALESCE(b.therapist_earnings, 0),
    'Therapist payout (backfill)',
    COALESCE(b.actual_end, b.updated_at, NOW()),
    NOW()
FROM bookings b
WHERE b.status = 'completed'
  AND b.therapist_earnings IS NOT NULL
  AND b.therapist_earnings > 0
  AND NOT EXISTS (
      SELECT 1 FROM ledger_entries le WHERE le.booking_id = b.booking_id AND le.category = 'payout'
  );
-- Migration: Add proof_url and status columns to ledger_entries
-- Purpose: Enable audit-ready expense tracking with optional approval workflow

-- Add ledger entry status enum
DO $$ BEGIN
    CREATE TYPE ledger_entry_status AS ENUM ('pending', 'approved', 'rejected');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add new columns
ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS proof_url TEXT,
ADD COLUMN IF NOT EXISTS status ledger_entry_status NOT NULL DEFAULT 'pending',
ADD COLUMN IF NOT EXISTS reviewed_by BIGINT REFERENCES users(user_id),
ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- CREATE INDEX IF NOT EXISTS for filtering by status
CREATE INDEX IF NOT EXISTS idx_ledger_entries_status ON ledger_entries(status);

-- Comment for documentation
COMMENT ON COLUMN ledger_entries.proof_url IS 'URL to receipt/invoice image for expense substantiation';
COMMENT ON COLUMN ledger_entries.status IS 'pending=awaiting review, approved=in the books, rejected=denied';
COMMENT ON COLUMN ledger_entries.reviewed_by IS 'User who approved or rejected the entry';
COMMENT ON COLUMN ledger_entries.reviewed_at IS 'When the entry was reviewed (approved/rejected)';
-- Add status_reason column to users table for ban/suspension audit trail
ALTER TABLE users ADD COLUMN IF NOT EXISTS status_reason TEXT;

-- Add index for users with non-active status (for admin queries)
CREATE INDEX IF NOT EXISTS idx_users_non_active_status ON users(account_status) WHERE account_status != 'active';
-- Migration: Add target_user_id for wallet tracking and settlement category
-- Purpose: Track owed balances per user (therapist) and record payouts.

-- 1. Add target_user_id column
ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS target_user_id BIGINT REFERENCES users(user_id);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_target_user ON ledger_entries(target_user_id);

-- 2. Add 'settlement' to ledger_category ENUM
-- Note: 'IF NOT EXISTS' for ENUM values requires PostgreSQL 12+.
-- If earlier version, we catch error. But assuming 12+ for this stack.
ALTER TYPE ledger_category ADD VALUE IF NOT EXISTS 'settlement';

-- 3. Backfill target_user_id for existing Payout entries from Bookings
UPDATE ledger_entries le
SET target_user_id = b.therapist_id
FROM bookings b
WHERE le.booking_id = b.booking_id
  AND le.category = 'payout'
  AND le.target_user_id IS NULL;

-- 4. Comment
COMMENT ON COLUMN ledger_entries.target_user_id IS 'The user who owns this balance (e.g., therapist for payouts).';
-- Add notes column to payments table
ALTER TABLE payments ADD COLUMN IF NOT EXISTS notes TEXT;

-- Add notes column to ledger_entries table
ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS notes TEXT;

-- Update payments status check constraint to include 'rejected'
-- First drop the old constraint, then add new one
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'expired', 'rejected'));
-- ============================================================================
-- MIGRATION 034: Fix Schema Mismatches for Integration Tests
-- ============================================================================
-- This migration adds missing columns to reconcile the DB with the Repository code.

-- 1. FIX BRANCHES TABLE
-- Add missing columns aligned with 001.sql and repository expectations
ALTER TABLE branches ADD COLUMN IF NOT EXISTS branch_name VARCHAR(150);
-- Sync existing data: if 'name' exists but 'branch_name' is null, copy 'name' to 'branch_name'
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'name'
  ) THEN
    UPDATE branches SET branch_name = name WHERE branch_name IS NULL AND name IS NOT NULL;
  END IF;
END $$;
ALTER TABLE branches ALTER COLUMN branch_name SET NOT NULL;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS address_line VARCHAR(255);
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'address'
  ) THEN
    UPDATE branches SET address_line = address WHERE address_line IS NULL AND address IS NOT NULL;
  END IF;
END $$;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS barangay VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS province VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS latitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS contact_no VARCHAR(20);
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'branches' AND column_name = 'phone'
  ) THEN
    UPDATE branches SET contact_no = phone WHERE contact_no IS NULL AND phone IS NOT NULL;
  END IF;
END $$;

-- 2. FIX ADMIN_ACTIONS TABLE
-- Add missing columns expected by AdminActionRepository
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_type VARCHAR(50);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_id INT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS old_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS new_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS performed_at TIMESTAMP;

-- Sync performed_at with created_at
UPDATE admin_actions SET performed_at = created_at WHERE performed_at IS NULL;

-- 3. FIX REFERRALS TABLE
-- Add missing columns expected by ReferralRepository
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referred_id INT;
-- referee_id in DB is likely the same as referred_id in Repository
UPDATE referrals SET referred_id = referee_id WHERE referred_id IS NULL AND referee_id IS NOT NULL;

ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referral_code VARCHAR(50);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS status VARCHAR(20);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS reward_earned BOOLEAN DEFAULT FALSE;
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;

-- 4. CREATE REFERRAL_REWARDS TABLE
-- This table was missing completely from the DB check
CREATE TABLE IF NOT EXISTS referral_rewards (
    reward_id SERIAL PRIMARY KEY,
    referral_id INT REFERENCES referrals(referral_id),
    user_id INT REFERENCES users(user_id),
    reward_type VARCHAR(50),
    reward_amount NUMERIC(10,2),
    status VARCHAR(20) DEFAULT 'pending',
    expires_at TIMESTAMP,
    redeemed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- ============================================================================
-- Migration 035: Shopping Cart Schema
-- ============================================================================
-- Adds server-side cart persistence for users.

-- Each user has one active cart.
CREATE TABLE IF NOT EXISTS carts (
    cart_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_carts_user_id ON carts(user_id);

-- Cart items store services with preferences, similar to booking details.
CREATE TABLE IF NOT EXISTS cart_items (
    cart_item_id SERIAL PRIMARY KEY,
    cart_id INT NOT NULL REFERENCES carts(cart_id) ON DELETE CASCADE,
    service_id INT NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    
    -- Guest and preferences
    guest_name VARCHAR(100) NOT NULL DEFAULT 'Self',
    duration_minutes INT NOT NULL DEFAULT 60,
    gender_preference VARCHAR(10) CHECK (gender_preference IN ('male', 'female', 'any')) DEFAULT 'any',
    pressure_preference VARCHAR(10) CHECK (pressure_preference IN ('soft', 'medium', 'hard')) DEFAULT 'medium',
    notes TEXT,
    
    -- Sequencing for group bookings
    sequence_number INT NOT NULL DEFAULT 0,
    start_condition VARCHAR(20) CHECK (start_condition IN ('fixed_time', 'after_previous')) DEFAULT 'fixed_time',
    
    -- Add-ons stored as JSONB array: [{"product_id": 1, "quantity": 2}, ...]
    addons JSONB DEFAULT '[]'::jsonb,
    
    date_added TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (duration_minutes > 0)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON cart_items(cart_id);

-- Trigger to update cart's updated_at when items change
CREATE OR REPLACE FUNCTION update_cart_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE carts SET updated_at = NOW() WHERE cart_id = NEW.cart_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_cart_items_update_cart_timestamp ON cart_items;
CREATE TRIGGER trg_cart_items_update_cart_timestamp
    AFTER INSERT OR UPDATE OR DELETE ON cart_items
    FOR EACH ROW
    EXECUTE FUNCTION update_cart_timestamp();
-- ============================================================================
-- Migration 036: Service Areas (Unified Location Governance)
-- ============================================================================
-- Fresh baseline keeps the canonical service-area DDL, indexes, trigger, and seed
-- in the first service-area section above. This historical consolidation point is
-- now intentionally a no-op to avoid duplicate active CREATE TABLE definitions.
-- Migration: 037_dynamic_travel_buffer.sql
-- Purpose: Add SQL functions for dynamic travel buffer calculation based on Haversine distance.

-- =============================================================================
-- 1. FUNCTION: calculate_distance_km
-- =============================================================================
-- Calculates the Great Circle distance between two points in kilometers.
CREATE OR REPLACE FUNCTION calculate_distance_km(lat1 float, lon1 float, lat2 float, lon2 float)
RETURNS float AS $$
DECLARE
    R float := 6371; -- Earth radius in km
    dLat float;
    dLon float;
    a float;
    c float;
BEGIN
    IF lat1 IS NULL OR lon1 IS NULL OR lat2 IS NULL OR lon2 IS NULL THEN
        RETURN NULL;
    END IF;

    dLat := radians(lat2 - lat1);
    dLon := radians(lon2 - lon1);
    
    -- Convert latitudes to radians for the formula
    a := sin(dLat/2) * sin(dLat/2) +
         sin(dLon/2) * sin(dLon/2) * cos(radians(lat1)) * cos(radians(lat2));
    c := 2 * atan2(sqrt(a), sqrt(1-a));
    RETURN R * c;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- =============================================================================
-- 2. FUNCTION: calculate_travel_buffer_minutes
-- =============================================================================
-- Returns the required buffer in minutes based on distance.
-- Logic:
--   < 0.5km: 0 minutes (Walking distance / Same building)
--   >= 0.5km: (Distance / Speed) + Setup Time
-- Assumptions:
--   - Average Speed: 20 km/h (Manila Traffic)
--   - Setup/Parking Time: 15 minutes
CREATE OR REPLACE FUNCTION calculate_travel_buffer_minutes(distance_km float)
RETURNS int AS $$
BEGIN
    -- If distance is unknown, assume they are far apart in different zones (Safe Default)
    IF distance_km IS NULL THEN
        RETURN 30; 
    END IF;

    IF distance_km < 0.5 THEN
        RETURN 0;
    END IF;

    -- Formula: (Dist / 20km/h * 60 mins) + 15 mins setup
    RETURN CEIL((distance_km / 20.0 * 60) + 15)::int;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
-- Migration: Add at_branch tracking for therapist location status
-- This enables the "Return to Branch" check-in feature

ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS at_branch BOOLEAN DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS last_location_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- CREATE INDEX IF NOT EXISTS for efficient filtering in matching queries
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_at_branch 
ON therapist_profiles(at_branch) 
WHERE deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN therapist_profiles.at_branch IS 'True if therapist is at their assigned branch. False when assigned to a booking. Set to true via check-in button.';
COMMENT ON COLUMN therapist_profiles.last_location_update IS 'Timestamp of last location status change (at_branch toggle)';
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
-- Migration 040: Architecture Upgrade (Bundles, Durable State)

-- 1. Create Booking Offer Items Table (For Bundles)
CREATE TABLE IF NOT EXISTS booking_offer_items (
    offer_id BIGINT NOT NULL REFERENCES booking_offers(offer_id) ON DELETE CASCADE,
    booking_id BIGINT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    estimated_earnings NUMERIC(10, 2) DEFAULT 0,
    PRIMARY KEY (offer_id, booking_id)
);

-- Add columns to booking_offers if they don't exist
ALTER TABLE booking_offers ADD COLUMN IF NOT EXISTS estimated_earnings NUMERIC(10, 2),
ADD COLUMN IF NOT EXISTS is_bundle BOOLEAN DEFAULT FALSE;

-- 2. Relax booking_offers.booking_id constraint
-- We keep the column for backward compatibility (and potentially as a "representative" ID),
-- but new logic might rely purely on the items table for multi-booking offers.
ALTER TABLE booking_offers ALTER COLUMN booking_id DROP NOT NULL;

-- 3. Add Workflow State to Assignment Queue (Durable Execution)
ALTER TABLE booking_assignment_queue ADD COLUMN IF NOT EXISTS workflow_state VARCHAR(50) DEFAULT 'init',
ADD COLUMN IF NOT EXISTS workflow_data JSONB DEFAULT '{}';

-- 4. Index for State Lookups?
-- (Optional: For now, the queue is small enough, fast key lookup by booking_id is primary)
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
-- Enable PostGIS for geospatial operations
CREATE EXTENSION IF NOT EXISTS postgis;

-- Rider Profiles
CREATE TABLE IF NOT EXISTS rider_profiles (
    rider_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    vehicle_type VARCHAR(50), -- 'motorcycle', 'car', 'suv'
    license_plate VARCHAR(20),
    license_number VARCHAR(50),
    is_online BOOLEAN DEFAULT FALSE,
    current_location GEOGRAPHY(POINT, 4326), -- SOTA: geography type for accurate distances
    last_location_update TIMESTAMPTZ,
    rating DECIMAL(3,2) DEFAULT 5.0,
    total_trips INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- GiST Index for fast geospatial lookups (SOTA 2026)
CREATE INDEX IF NOT EXISTS idx_rider_location ON rider_profiles USING GIST(current_location);
CREATE INDEX IF NOT EXISTS idx_rider_online ON rider_profiles(is_online) WHERE is_online = true;

-- Rides Table
CREATE TABLE IF NOT EXISTS rides (
    ride_id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT REFERENCES rider_profiles(rider_id),
    passenger_id BIGINT NOT NULL REFERENCES users(user_id), -- Therapist
    booking_id BIGINT REFERENCES bookings(booking_id), -- Optional link to massage booking
    
    -- Pickup (Client location)
    pickup_lat DECIMAL(10, 7) NOT NULL,
    pickup_long DECIMAL(10, 7) NOT NULL,
    pickup_address TEXT,
    
    -- Dropoff (Homebase or home)
    dropoff_lat DECIMAL(10, 7) NOT NULL,
    dropoff_long DECIMAL(10, 7) NOT NULL,
    dropoff_address TEXT,
    
    -- Pricing (Snapshot for historical accuracy)
    distance_km DECIMAL(6,2),
    pricing_snapshot JSONB, -- {base_rate, per_km_rate, surge_multiplier, final_fare}
    
    -- Status Flow
    status VARCHAR(30) DEFAULT 'pending', 
    -- 'pending' -> 'offered' -> 'accepted' -> 'arrived_pickup' -> 'in_progress' -> 'completed' / 'cancelled'
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    offered_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    arrived_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancellation_reason TEXT,
    
    retry_count INT NOT NULL DEFAULT 0,
    last_retried_at TIMESTAMPTZ,
    scheduled_for TIMESTAMPTZ,
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rides_rider ON rides(rider_id);
CREATE INDEX IF NOT EXISTS idx_rides_passenger ON rides(passenger_id);
CREATE INDEX IF NOT EXISTS idx_rides_status ON rides(status);

-- Composite index for GetUnmatchedRidesForRetry
CREATE INDEX IF NOT EXISTS idx_rides_retry_lookup ON rides (status, rider_id, retry_count, last_retried_at)
  WHERE status = 'pending' AND rider_id IS NULL;

-- Partial index for schedule-aware rider filtering
CREATE INDEX IF NOT EXISTS idx_rides_active_schedule ON rides (rider_id, scheduled_for)
  WHERE status IN ('accepted', 'arrived_pickup', 'in_progress', 'arrived_dropoff') AND scheduled_for IS NOT NULL;

-- Ride Offers
CREATE TABLE IF NOT EXISTS ride_offers (
    offer_id BIGSERIAL PRIMARY KEY,
    ride_id BIGINT NOT NULL REFERENCES rides(ride_id) ON DELETE CASCADE,
    rider_id BIGINT NOT NULL REFERENCES rider_profiles(rider_id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    UNIQUE(ride_id, rider_id)
);

CREATE INDEX IF NOT EXISTS idx_ride_offers_ride_id ON ride_offers(ride_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_rider_id ON ride_offers(rider_id);
CREATE INDEX IF NOT EXISTS idx_ride_offers_status ON ride_offers(status);
CREATE INDEX IF NOT EXISTS idx_ride_offers_expires_at ON ride_offers(expires_at) WHERE status = 'pending';

-- Ride Pricing Configuration (Admin-Configurable)
CREATE TABLE IF NOT EXISTS ride_pricing_config (
    config_id SERIAL PRIMARY KEY,
    config_key VARCHAR(50) UNIQUE DEFAULT 'default',
    base_distance_km DECIMAL(4,2) DEFAULT 3.0,
    base_rate DECIMAL(8,2) DEFAULT 50.0,
    per_km_rate DECIMAL(8,2) DEFAULT 10.0,
    per_100m_rate DECIMAL(8,2) DEFAULT 1.0, -- Granular pricing (.1 km)
    min_fare DECIMAL(8,2) DEFAULT 50.0,
    max_fare DECIMAL(8,2) DEFAULT 150.0,
    surge_enabled BOOLEAN DEFAULT FALSE,
    dispatch_buffer_minutes INTEGER DEFAULT 30,
    default_vehicle_type VARCHAR(50) DEFAULT 'motorcycle',
    surge_multiplier DECIMAL(3,2) DEFAULT 1.0, -- SOTA: Dynamic pricing support
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO ride_pricing_config (config_key) VALUES ('default') ON CONFLICT DO NOTHING;
-- ============================================================================
-- Migration 041: Therapist Wallet System
-- ============================================================================
-- Provides real-time balance tracking for therapist earnings, payouts, and cash advances.
-- Integrates with the existing ledger system for full audit trail.

-- =============================================================================
-- 1. THERAPIST WALLETS (Balance Tracking)
-- =============================================================================
CREATE TABLE IF NOT EXISTS therapist_wallets (
    wallet_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE RESTRICT,
    
    -- Balance tracking
    available_balance NUMERIC(12,2) NOT NULL DEFAULT 0,  -- Ready for withdrawal
    pending_balance NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Held for 24h after booking
    
    -- Lifetime totals (for dashboard/reporting)
    total_earned NUMERIC(12,2) NOT NULL DEFAULT 0,       -- Sum of all earnings
    total_withdrawn NUMERIC(12,2) NOT NULL DEFAULT 0,    -- Sum of all payouts
    total_advances NUMERIC(12,2) NOT NULL DEFAULT 0,     -- Sum of all cash advances
    
    -- Payout settings
    minimum_payout NUMERIC(12,2) NOT NULL DEFAULT 500,   -- Minimum withdrawal amount
    last_payout_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Balance can go negative due to cash advances
    CHECK (pending_balance >= 0)
);

-- Index for fast therapist lookup
CREATE INDEX IF NOT EXISTS idx_therapist_wallets_therapist ON therapist_wallets(therapist_id);

-- =============================================================================
-- 2. WALLET TRANSACTIONS (Audit Trail)
-- =============================================================================
-- Every balance change creates a transaction record for full traceability.
CREATE TABLE IF NOT EXISTS wallet_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    ledger_entry_id BIGINT REFERENCES ledger_entries(entry_id) ON DELETE SET NULL,
    
    -- Transaction details
    type VARCHAR(30) NOT NULL CHECK (type IN (
        'earning',           -- From completed booking
        'earning_released',  -- Moved from pending to available
        'payout',            -- Withdrawal to external account
        'cash_advance',      -- Pre-payment to therapist
        'advance_repayment', -- Deducted from earnings to repay advance
        'adjustment',        -- Manual correction by admin
        'refund_clawback'    -- Returned due to client refund
    )),
    
    amount NUMERIC(12,2) NOT NULL,              -- Positive for credit, negative for debit
    balance_after NUMERIC(12,2) NOT NULL,       -- Snapshot of available_balance after txn
    pending_after NUMERIC(12,2) NOT NULL DEFAULT 0, -- Snapshot of pending_balance after txn
    
    description TEXT,
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL, -- Admin who processed (if applicable)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_wallet_txns_wallet ON wallet_transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_booking ON wallet_transactions(booking_id);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_type ON wallet_transactions(type);
CREATE INDEX IF NOT EXISTS idx_wallet_txns_created ON wallet_transactions(created_at DESC);

-- =============================================================================
-- 3. PAYOUT REQUESTS (Withdrawal Queue)
-- =============================================================================
-- Tracks therapist requests to withdraw funds.
CREATE TABLE IF NOT EXISTS payout_requests (
    request_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    amount NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    payout_method VARCHAR(30) NOT NULL CHECK (payout_method IN ('gcash', 'bank_transfer', 'cash')),
    account_details JSONB, -- {account_name, account_number, bank_name} for bank; {phone} for gcash
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending',     -- Awaiting admin approval
        'approved',    -- Approved, processing payment
        'completed',   -- Payment sent
        'rejected',    -- Rejected by admin
        'cancelled'    -- Cancelled by therapist
    )),
    
    -- Processing info
    processed_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    transaction_reference TEXT, -- External payment reference (bank txn, GCash ref)
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payout_requests_wallet ON payout_requests(wallet_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_therapist ON payout_requests(therapist_id);
CREATE INDEX IF NOT EXISTS idx_payout_requests_status ON payout_requests(status);

-- =============================================================================
-- 4. CASH ADVANCE RECORDS
-- =============================================================================
-- Tracks cash advances given to therapists (to be repaid from future earnings).
CREATE TABLE IF NOT EXISTS cash_advances (
    advance_id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES therapist_wallets(wallet_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    original_amount NUMERIC(12,2) NOT NULL CHECK (original_amount > 0),
    remaining_balance NUMERIC(12,2) NOT NULL CHECK (remaining_balance >= 0),
    repayment_rate NUMERIC(5,2) NOT NULL DEFAULT 50.00, -- % of each earning to deduct (e.g., 50%)
    
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN (
        'active',    -- Being repaid from earnings
        'paid_off',  -- Fully repaid
        'written_off' -- Admin wrote off the balance
    )),
    
    -- Approval info
    approved_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    reason TEXT,
    
    paid_off_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cash_advances_wallet ON cash_advances(wallet_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_therapist ON cash_advances(therapist_id);
CREATE INDEX IF NOT EXISTS idx_cash_advances_status ON cash_advances(status) WHERE status = 'active';

-- =============================================================================
-- 5. AUTO-CREATE WALLET ON THERAPIST PROFILE CREATION
-- =============================================================================
CREATE OR REPLACE FUNCTION create_therapist_wallet()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO therapist_wallets (therapist_id)
    VALUES (NEW.therapist_id)
    ON CONFLICT (therapist_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_create_wallet_on_therapist ON therapist_profiles;
CREATE TRIGGER trg_create_wallet_on_therapist
    AFTER INSERT ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION create_therapist_wallet();

-- =============================================================================
-- 6. BACKFILL: Create wallets for existing therapists
-- =============================================================================
INSERT INTO therapist_wallets (therapist_id)
SELECT therapist_id FROM therapist_profiles
WHERE therapist_id NOT IN (SELECT therapist_id FROM therapist_wallets)
ON CONFLICT (therapist_id) DO NOTHING;

-- Backfill lifetime totals from ledger
-- Note: Only includes approved entries if status column exists (migration 030)
UPDATE therapist_wallets w
SET total_earned = COALESCE((
    SELECT SUM(amount) FROM ledger_entries le 
    WHERE le.category = 'payout' 
    AND le.entry_type = 'debit'
    AND (
        NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'ledger_entries' AND column_name = 'status'
        )
        OR le.status::text = 'approved'
    )
    AND le.booking_id IN (SELECT booking_id FROM bookings WHERE therapist_id = w.therapist_id)
), 0);

-- Set available_balance to total_earned (since no withdrawals recorded yet in new system)
UPDATE therapist_wallets
SET available_balance = total_earned
WHERE total_earned > 0;
-- Add 'rider' role to users table check constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check 
    CHECK (role IN ('client', 'therapist', 'admin', 'super_admin', 'rider'));
-- Migration 043: Add ride_type and therapist location support for logistics integration
-- This migration supports the Event-Driven Rider Integration feature

-- Add ride_type to rides table to distinguish outbound vs return trips
ALTER TABLE rides ADD COLUMN IF NOT EXISTS ride_type VARCHAR(20) DEFAULT 'outbound'
CHECK (ride_type IN ('outbound', 'return'));

-- CREATE INDEX IF NOT EXISTS for ride-booking linkage queries
CREATE INDEX IF NOT EXISTS idx_rides_booking_id ON rides(booking_id) WHERE booking_id IS NOT NULL;

-- Create composite index for querying rides by type and status
CREATE INDEX IF NOT EXISTS idx_rides_type_status ON rides(ride_type, status);

-- Add home_address_id for therapists (future enhancement, optional for now)
-- Therapists can have a default pickup location separate from their branch
ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS home_address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL;

-- Add index for therapist location resolution
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_home_address ON therapist_profiles(home_address_id) 
WHERE home_address_id IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN rides.ride_type IS 'Type of ride: outbound (therapist to client) or return (client back to therapist home/branch)';
COMMENT ON COLUMN therapist_profiles.home_address_id IS 'Therapist home address for ride pickups (optional, defaults to branch_id location if null)';
COMMENT ON INDEX idx_rides_booking_id IS 'Efficiently query rides associated with a specific booking';
COMMENT ON INDEX idx_rides_type_status IS 'Optimize queries filtering by ride type and status (e.g., pending return rides)';
-- ============================================================================
-- RIDER WALLET SYSTEM
-- ============================================================================

-- Rider Wallets (mirrors therapist_wallets design)
CREATE TABLE IF NOT EXISTS rider_wallets (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    balance_cents INT NOT NULL DEFAULT 0,
    total_earned_cents INT NOT NULL DEFAULT 0,
    total_withdrawn_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_walletbalance_non_negative CHECK (balance_cents >= 0),
    CONSTRAINT rider_wallet_totals_consistent CHECK (total_earned_cents >= total_withdrawn_cents)
);

COMMENT ON TABLE rider_wallets IS 'Tracks rider earnings and payout balances';
COMMENT ON COLUMN rider_wallets.balance_cents IS 'Current available balance in cents';
COMMENT ON COLUMN rider_wallets.total_earned_cents IS 'Lifetime earnings from all rides';
COMMENT ON COLUMN rider_wallets.total_withdrawn_cents IS 'Total amount withdrawn via payouts';

-- Rider Payout Methods
CREATE TABLE IF NOT EXISTS rider_payout_methods (
    id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method_type VARCHAR(20) NOT NULL CHECK (method_type IN ('bank', 'gcash', 'paymaya', 'grabpay')),
    provider_name VARCHAR(100) NOT NULL,
    account_number VARCHAR(100) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rider_payout_methods_rider ON rider_payout_methods(rider_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rider_payout_methods_default ON rider_payout_methods(rider_id) WHERE is_default = TRUE;

-- Rider Transactions (similar to therapist ledger_entries)
CREATE TABLE IF NOT EXISTS rider_transactions (
    transaction_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('ride_earning', 'payout', 'adjustment', 'bonus')),
    amount_cents INT NOT NULL,
    ride_id INT REFERENCES rides(ride_id) ON DELETE SET NULL,
    payout_method_id INT REFERENCES rider_payout_methods(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    
    CONSTRAINT rider_transaction_amount_non_zero CHECK (amount_cents != 0)
);

COMMENT ON TABLE rider_transactions IS 'Transaction history for rider wallet operations';
COMMENT ON COLUMN rider_transactions.transaction_type IS 'Type of transaction: ride_earning (credit), payout (debit), adjustment (admin), bonus (credit)';
COMMENT ON COLUMN rider_transactions.amount_cents IS 'Transaction amount in cents (positive for credit, negative for debit)';
COMMENT ON COLUMN rider_transactions.status IS 'Transaction status for async operations like payouts';

CREATE INDEX IF NOT EXISTS idx_rider_transactions_rider ON rider_transactions(rider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_ride ON rider_transactions(ride_id);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_status ON rider_transactions(status) WHERE status = 'pending';

-- ============================================================================
-- RIDER PERFORMANCE METRICS
-- ============================================================================

CREATE TABLE IF NOT EXISTS rider_performance_metrics (
    rider_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    total_offers_received INT NOT NULL DEFAULT 0,
    total_rides_accepted INT NOT NULL DEFAULT 0,
    total_rides_completed INT NOT NULL DEFAULT 0,
    total_rides_cancelled INT NOT NULL DEFAULT 0,
    acceptance_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    completion_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    average_rating DECIMAL(3,2) DEFAULT NULL,
    total_ratings INT NOT NULL DEFAULT 0,
    rating_sum INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_acceptance_rate_valid CHECK (acceptance_rate >= 0 AND acceptance_rate <= 100),
    CONSTRAINT rider_completion_rate_valid CHECK (completion_rate >= 0 AND completion_rate <= 100),
    CONSTRAINT rider_average_rating_valid CHECK (average_rating IS NULL OR (average_rating >= 1 AND average_rating <= 5))
);

COMMENT ON TABLE rider_performance_metrics IS 'Tracks rider performance and ratings for quality control';
COMMENT ON COLUMN rider_performance_metrics.acceptance_rate IS 'Percentage of offers accepted (rides_accepted / offers_received * 100)';
COMMENT ON COLUMN rider_performance_metrics.completion_rate IS 'Percentage of rides completed (rides_completed / rides_accepted * 100)';
COMMENT ON COLUMN rider_performance_metrics.average_rating IS 'Average passenger rating (1-5 stars)';

-- ============================================================================
-- SAFETY FEATURES
-- ============================================================================

-- Emergency Contacts for Riders
CREATE TABLE IF NOT EXISTS rider_emergency_contacts (
    contact_id SERIAL PRIMARY KEY,
    rider_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    full_name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    relationship VARCHAR(50),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_emergency_phone_format CHECK (phone_number ~ '^\+?[0-9]{10,15}$')
);

COMMENT ON TABLE rider_emergency_contacts IS 'Emergency contacts for rider safety features (SOS, trip sharing)';
COMMENT ON COLUMN rider_emergency_contacts.is_primary IS 'Primary contact receives alerts first';

CREATE INDEX IF NOT EXISTS idx_rider_emergency_contacts_rider ON rider_emergency_contacts(rider_id);

-- ============================================================================
-- EXTEND RIDES TABLE
-- ============================================================================

-- Add rider earnings column to rides table
ALTER TABLE rides ADD COLUMN IF NOT EXISTS rider_earnings_cents INT;

COMMENT ON COLUMN rides.rider_earnings_cents IS 'Amount credited to rider wallet upon ride completion (calculated from fare)';

-- ============================================================================
-- TRIGGERS FOR AUTO-CALCULATION
-- ============================================================================

-- Trigger to update rider wallet when ride earnings are recorded
CREATE OR REPLACE FUNCTION update_rider_wallet_on_earning()
RETURNS TRIGGER AS $$
BEGIN
    -- Only process if rider_earnings_cents is set and status is completed
    IF NEW.rider_earnings_cents IS NOT NULL AND NEW.status = 'completed' AND 
       (OLD.status IS NULL OR OLD.status != 'completed') THEN
        
        -- Ensure wallet exists
        INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents)
        VALUES (NEW.rider_id, 0, 0)
        ON CONFLICT (rider_id) DO NOTHING;
        
        -- Update wallet
        UPDATE rider_wallets
        SET 
            balance_cents = balance_cents + NEW.rider_earnings_cents,
            total_earned_cents = total_earned_cents + NEW.rider_earnings_cents,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
        -- Create transaction record
        INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, ride_id, status, description)
        VALUES (
            NEW.rider_id,
            'ride_earning',
            NEW.rider_earnings_cents,
            NEW.ride_id,
            'completed',
            FORMAT('Earnings from ride #%s', NEW.ride_id)
        );
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_rider_wallet ON rides;
DROP TRIGGER IF EXISTS trigger_update_rider_wallet ON rides;
CREATE TRIGGER trigger_update_rider_wallet
AFTER UPDATE ON rides
FOR EACH ROW
EXECUTE FUNCTION update_rider_wallet_on_earning();

-- Trigger to update performance metrics when rider accepts/completes rides
CREATE OR REPLACE FUNCTION update_rider_performance_metrics()
RETURNS TRIGGER AS $$
DECLARE
    v_acceptance_rate DECIMAL(5,2);
    v_completion_rate DECIMAL(5,2);
BEGIN
    -- Ensure metrics row exists
    INSERT INTO rider_performance_metrics (rider_id)
    VALUES (NEW.rider_id)
    ON CONFLICT (rider_id) DO NOTHING;
    
    -- Update based on status change
    IF NEW.status = 'accepted' AND (OLD.status IS NULL OR OLD.status = 'pending') THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_accepted = total_rides_accepted + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
    ELSIF NEW.status = 'completed' AND OLD.status != 'completed' THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_completed = total_rides_completed + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
        
    ELSIF NEW.status = 'cancelled' AND OLD.status NOT IN ('cancelled', 'completed') THEN
        UPDATE rider_performance_metrics
        SET 
            total_rides_cancelled = total_rides_cancelled + 1,
            updated_at = NOW()
        WHERE rider_id = NEW.rider_id;
    END IF;
    
    -- Recalculate rates
    UPDATE rider_performance_metrics
    SET 
        acceptance_rate = CASE 
            WHEN total_offers_received > 0 
            THEN (total_rides_accepted::DECIMAL / total_offers_received * 100)
            ELSE 0 
        END,
        completion_rate = CASE 
            WHEN total_rides_accepted > 0 
            THEN (total_rides_completed::DECIMAL / total_rides_accepted * 100)
            ELSE 0 
        END
    WHERE rider_id = NEW.rider_id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_rider_performance ON rides;
DROP TRIGGER IF EXISTS trigger_update_rider_performance ON rides;
CREATE TRIGGER trigger_update_rider_performance
AFTER UPDATE ON rides
FOR EACH ROW
WHEN (NEW.rider_id IS NOT NULL)
EXECUTE FUNCTION update_rider_performance_metrics();

-- ============================================================================
-- INITIAL DATA / SEED
-- ============================================================================

-- Create wallet and metrics for existing riders (retroactive)
INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents)
SELECT DISTINCT u.user_id, 0, 0
FROM users u
WHERE u.role = 'rider'
ON CONFLICT (rider_id) DO NOTHING;

INSERT INTO rider_performance_metrics (rider_id)
SELECT DISTINCT u.user_id
FROM users u
WHERE u.role = 'rider'
ON CONFLICT (rider_id) DO NOTHING;
-- Migration 045: Fix Therapist Services Cascade Deletion
-- Changes the foreign key constraint on therapist_services(service_id) to ON DELETE CASCADE
-- This allows deleting a service to automatically remove it from all therapists' profiles

ALTER TABLE therapist_services
DROP CONSTRAINT IF EXISTS therapist_services_service_id_fkey;

ALTER TABLE therapist_services DROP CONSTRAINT IF EXISTS therapist_services_service_id_fkey;
ALTER TABLE therapist_services ADD CONSTRAINT therapist_services_service_id_fkey
    FOREIGN KEY (service_id)
    REFERENCES services(service_id)
    ON DELETE CASCADE;
ALTER TABLE call_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_addons ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_offer_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE reviews ENABLE ROW LEVEL SECURITY;
-- Removed RLS for non-existent table: ALTER TABLE therapist_service_pressures ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE client_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_promotions ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE live_locations ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE services ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_extension_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE admin_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE rider_emergency_contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_services ENABLE ROW LEVEL SECURITY;
ALTER TABLE bookings ENABLE ROW LEVEL SECURITY;
ALTER TABLE referral_rewards ENABLE ROW LEVEL SECURITY;
ALTER TABLE referrals ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_rate_limits ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE emergency_alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE cart_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE rider_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE cash_advances ENABLE ROW LEVEL SECURITY;
ALTER TABLE ride_pricing_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE favorite_therapists ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_ticket_attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE client_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE payout_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE addresses ENABLE ROW LEVEL SECURITY;
ALTER TABLE branches ENABLE ROW LEVEL SECURITY;
ALTER TABLE rides ENABLE ROW LEVEL SECURITY;
ALTER TABLE promotions ENABLE ROW LEVEL SECURITY;
ALTER TABLE area_coverage_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_areas ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_assignment_queue ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE rider_performance_metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE carts ENABLE ROW LEVEL SECURITY;
ALTER TABLE wallet_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE rider_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_blocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_auth_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE rider_wallets ENABLE ROW LEVEL SECURITY;


-- ============================================================================
-- Consolidated from 063_add_target_role_to_ledger.sql
-- ============================================================================
-- Migration 063: Add target_role to ledger_entries for unified payout tracking
-- Apply manually: psql -d <db> -f 063_add_target_role_to_ledger.sql

ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS target_role VARCHAR(20);

-- Backfill: existing entries with a target_user_id are therapist payouts/settlements
UPDATE ledger_entries
SET target_role = 'therapist'
WHERE target_user_id IS NOT NULL AND target_role IS NULL;

-- Composite indexes for role-aware balance queries
CREATE INDEX IF NOT EXISTS idx_ledger_target_role_user
    ON ledger_entries(target_role, target_user_id, category, voided);

CREATE INDEX IF NOT EXISTS idx_ledger_date_role
    ON ledger_entries(entry_date, target_role, category, voided);


-- ============================================================================
-- Consolidated from 064_create_legal_documents.sql
-- ============================================================================
-- Migration 064: Create legal_documents table and seed default legal content

CREATE TABLE IF NOT EXISTS legal_documents (
    doc_key VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content_markdown TEXT NOT NULL,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO legal_documents (doc_key, title, content_markdown, version, effective_at, updated_at)
VALUES
(
    'privacy-policy',
    'Privacy Policy',
    $$# Privacy Policy

This Privacy Policy explains how Relaxation Hub collects, uses, and protects personal information.

## Information We Collect
- Account information (name, email, phone, and profile details)
- Location information required for rider operations
- Device and notification token data

## How We Use Data
- Deliver core platform functionality
- Improve reliability, safety, and support workflows
- Send service-related communications

## Contact
For privacy concerns, please contact support through the app.$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'terms-of-service',
    'Terms of Service',
    $$# Terms of Service

By using Relaxation Hub, you agree to comply with platform policies and applicable laws.

## Rider Responsibilities
- Keep account information accurate
- Follow ride and safety procedures
- Avoid fraudulent or abusive behavior

## Enforcement
Accounts may be restricted for policy violations.

## Changes
These terms may be updated from time to time.$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'about',
    'About Relaxation Hub',
    $$# About Relaxation Hub

Relaxation Hub connects clients, therapists, and riders through a coordinated service platform.

## Mission
Deliver dependable, safe, and high-quality wellness support.

## Support
Need help? Open a support ticket inside the app.$$,
    '1.0.0',
    NOW(),
    NOW()
)
ON CONFLICT (doc_key) DO UPDATE
SET
    title = EXCLUDED.title,
    content_markdown = EXCLUDED.content_markdown,
    version = EXCLUDED.version,
    effective_at = EXCLUDED.effective_at,
    updated_at = NOW();


-- ============================================================================
-- Consolidated from 065_seed_content_policy_keys.sql
-- ============================================================================
-- Migration 065: Seed policy keys used by /api/v1/content/{key}

INSERT INTO legal_documents (doc_key, title, content_markdown, version, effective_at, updated_at)
VALUES
(
    'terms_and_conditions',
    'Terms and Conditions',
    $$<h1>Terms and Conditions</h1>
<p>By booking services with Relaxation Hub, you agree to these terms and conditions.</p>
<h2>1. Service Scope</h2>
<p>Services are fulfilled based on the booking details you provide. Please review your selections before confirming.</p>
<h2>2. Client Responsibilities</h2>
<p>Provide accurate location, contact, and special instructions to help ensure successful service delivery.</p>
<h2>3. Cancellations and No-Shows</h2>
<p>Late cancellations and no-shows may be subject to policy enforcement under platform rules.</p>
<h2>4. Liability</h2>
<p>Relaxation Hub is not liable for delays caused by force majeure events, traffic disruptions, or other events beyond reasonable control.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'privacy_policy',
    'Privacy Policy',
    $$<h1>Privacy Policy</h1>
<p>Relaxation Hub values your privacy. We only collect information needed to operate and improve our services.</p>
<h2>1. Data We Collect</h2>
<p>We may collect account, booking, location, and device-related information required to provide service functionality.</p>
<h2>2. Data Usage</h2>
<p>Data is used for booking fulfillment, safety operations, communications, support, and service improvements.</p>
<h2>3. Data Protection</h2>
<p>We use reasonable security controls to protect your personal data and restrict unauthorized access.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'refund_policy',
    'Refund Policy',
    $$<h1>Refund Policy</h1>
<p>Relaxation Hub reviews refund requests fairly based on booking records and reported incidents.</p>
<h2>1. Request Window</h2>
<p>Please submit refund concerns as soon as possible after service completion or issue occurrence.</p>
<h2>2. Eligibility</h2>
<p>Approved refunds depend on verification, booking details, and policy compliance.</p>
<h2>3. Resolution</h2>
<p>Depending on the case, resolution may include partial refund, full refund, credit, or other remediation.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
)
ON CONFLICT (doc_key) DO NOTHING;


-- ============================================================================
-- Consolidated from 066_create_moderation_blocks.sql
-- ============================================================================
-- Migration 066: Global moderation block list for admin/super-admin tools

CREATE TABLE IF NOT EXISTS moderation_blocks (
    block_id BIGSERIAL PRIMARY KEY,
    blocked_user_id BIGINT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    blocked_by_admin_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_moderation_blocks_active_updated_at
    ON moderation_blocks (updated_at DESC)
    WHERE removed_at IS NULL;


-- ============================================================================
-- Consolidated from 067_create_day_view_therapist_orders.sql
-- ============================================================================
-- Migration 067: Persisted Day View therapist ordering by business day and scope.

CREATE TABLE IF NOT EXISTS day_view_therapist_orders (
    order_id BIGSERIAL PRIMARY KEY,
    view_key TEXT NOT NULL,
    business_date DATE NOT NULL,
    therapist_ids BIGINT[] NOT NULL DEFAULT '{}',
    source TEXT NOT NULL CHECK (source IN ('auto', 'manual')),
    updated_by_admin_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (view_key, business_date)
);

CREATE INDEX IF NOT EXISTS idx_day_view_therapist_orders_view_date
    ON day_view_therapist_orders (view_key, business_date DESC);


-- ============================================================================
-- Consolidated from 068_create_applicant_applications.sql
-- ============================================================================
-- Migration 068: Applicant applications for rider/therapist onboarding.

CREATE TABLE IF NOT EXISTS applicant_applications (
    application_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_role VARCHAR(32) NOT NULL,
    position_applied VARCHAR(128) NOT NULL,
    preferred_branch_id BIGINT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    preferred_branch_label VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    answers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    attachments_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by_admin_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    review_notes TEXT,
    CONSTRAINT chk_applicant_applications_target_role CHECK (target_role IN ('therapist', 'rider')),
    CONSTRAINT chk_applicant_applications_status CHECK (status IN ('pending', 'approved', 'rejected', 'needs_followup'))
);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_status_submitted
    ON applicant_applications (status, submitted_at DESC);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_role_status
    ON applicant_applications (target_role, status);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_user_id
    ON applicant_applications (user_id);


-- ============================================================================
-- Consolidated from 069_enable_pg_trgm.sql
-- ============================================================================
-- Migration 069: Enable trigram support for ILIKE text search acceleration.

CREATE EXTENSION IF NOT EXISTS pg_trgm;


-- ============================================================================
-- Consolidated from 070_add_query_performance_indexes.sql
-- ============================================================================
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


-- ============================================================================
-- Consolidated from 071_create_booking_referrals.sql
-- ============================================================================
CREATE TABLE IF NOT EXISTS booking_referrals (
    booking_id BIGINT PRIMARY KEY REFERENCES bookings(booking_id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    other_notes TEXT,
    created_by_user_id BIGINT NOT NULL REFERENCES users(user_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_booking_referrals_created_at ON booking_referrals(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_booking_referrals_source ON booking_referrals(source);


-- ============================================================================
-- Consolidated from 072_replace_psgc_with_area_key.sql
-- ============================================================================
-- Fresh baseline now creates area_key directly; historical rename blocks are no longer needed here.


-- ============================================================================
-- Consolidated from 073_ensure_service_area_schema.sql
-- ============================================================================
-- Fresh baseline already defines the canonical area_key-based service-area schema
-- in the first service-area section above, so this historical safety migration is
-- intentionally a no-op here.


-- ============================================================================
-- Consolidated from 074_add_promotion_applies_to.sql
-- ============================================================================
ALTER TABLE promotions
ADD COLUMN IF NOT EXISTS applies_to TEXT NOT NULL DEFAULT 'full_basket';

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'promotions_applies_to_check'
	) THEN
		ALTER TABLE promotions
		ADD CONSTRAINT promotions_applies_to_check
		CHECK (applies_to IN ('full_basket', 'services_only'));
	END IF;
END $$;


-- ============================================================================
-- Consolidated from 075_add_rider_wallet_runtime_foundations.sql
-- ============================================================================
CREATE TABLE IF NOT EXISTS rider_wallets (
    rider_id BIGINT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    balance_cents INTEGER NOT NULL DEFAULT 0,
    total_earned_cents INTEGER NOT NULL DEFAULT 0,
    total_withdrawn_cents INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rider_performance_metrics (
    rider_id BIGINT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    total_offers_received INTEGER NOT NULL DEFAULT 0,
    total_rides_accepted INTEGER NOT NULL DEFAULT 0,
    total_rides_completed INTEGER NOT NULL DEFAULT 0,
    total_rides_cancelled INTEGER NOT NULL DEFAULT 0,
    acceptance_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    completion_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    average_rating DOUBLE PRECISION,
    total_ratings INTEGER NOT NULL DEFAULT 0,
    rating_sum INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rider_payout_methods (
    id SERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method_type TEXT NOT NULL,
    provider_name TEXT NOT NULL,
    account_number TEXT NOT NULL,
    account_name TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'rider_payout_methods_method_type_check'
    ) THEN
        ALTER TABLE rider_payout_methods
        ADD CONSTRAINT rider_payout_methods_method_type_check
        CHECK (method_type IN ('bank', 'gcash', 'paymaya', 'grabpay'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS rider_transactions (
    transaction_id SERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    transaction_type TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    ride_id BIGINT REFERENCES rides(ride_id) ON DELETE SET NULL,
    payout_method_id INTEGER REFERENCES rider_payout_methods(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'rider_transactions_type_check'
    ) THEN
        ALTER TABLE rider_transactions
        ADD CONSTRAINT rider_transactions_type_check
        CHECK (transaction_type IN ('ride_earning', 'payout', 'adjustment', 'bonus'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'rider_transactions_status_check'
    ) THEN
        ALTER TABLE rider_transactions
        ADD CONSTRAINT rider_transactions_status_check
        CHECK (status IN ('pending', 'completed', 'failed'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS rider_emergency_contacts (
    contact_id SERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    full_name TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    relationship TEXT,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'rider_emergency_contacts'
          AND column_name = 'contact_name'
    )
    AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'rider_emergency_contacts'
          AND column_name = 'full_name'
    ) THEN
        ALTER TABLE rider_emergency_contacts RENAME COLUMN contact_name TO full_name;
    END IF;
END $$;

ALTER TABLE rides
ADD COLUMN IF NOT EXISTS rider_earnings_cents INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_rider_transactions_rider_created_at
    ON rider_transactions (rider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rider_transactions_created_at
    ON rider_transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rider_payout_methods_rider_id
    ON rider_payout_methods (rider_id);
CREATE INDEX IF NOT EXISTS idx_rider_emergency_contacts_rider_id
    ON rider_emergency_contacts (rider_id);


-- ============================================================================
-- Consolidated from 076_enforce_rider_default_primary_invariants.sql
-- ============================================================================
-- Canonicalize duplicate payout defaults before enforcing uniqueness.
WITH ranked_defaults AS (
    SELECT
        id,
        rider_id,
        ROW_NUMBER() OVER (PARTITION BY rider_id ORDER BY updated_at DESC, id DESC) AS rn
    FROM rider_payout_methods
    WHERE is_default = TRUE
)
UPDATE rider_payout_methods rpm
SET is_default = FALSE,
    updated_at = NOW()
FROM ranked_defaults rd
WHERE rpm.id = rd.id
  AND rd.rn > 1;

-- Ensure riders with payout methods have one default.
WITH candidates AS (
    SELECT DISTINCT ON (rider_id)
        id,
        rider_id
    FROM rider_payout_methods
    ORDER BY rider_id, updated_at DESC, id DESC
)
UPDATE rider_payout_methods rpm
SET is_default = TRUE,
    updated_at = NOW()
FROM candidates c
WHERE rpm.id = c.id
  AND NOT EXISTS (
      SELECT 1
      FROM rider_payout_methods existing
      WHERE existing.rider_id = c.rider_id
        AND existing.is_default = TRUE
  );

CREATE UNIQUE INDEX IF NOT EXISTS uq_rider_payout_methods_single_default
    ON rider_payout_methods (rider_id)
    WHERE is_default = TRUE;

-- Canonicalize duplicate emergency-contact primaries before enforcing uniqueness.
WITH ranked_primaries AS (
    SELECT
        contact_id,
        rider_id,
        ROW_NUMBER() OVER (PARTITION BY rider_id ORDER BY updated_at DESC, contact_id DESC) AS rn
    FROM rider_emergency_contacts
    WHERE is_primary = TRUE
)
UPDATE rider_emergency_contacts rec
SET is_primary = FALSE,
    updated_at = NOW()
FROM ranked_primaries rp
WHERE rec.contact_id = rp.contact_id
  AND rp.rn > 1;

-- Ensure riders with emergency contacts have one primary.
WITH candidates AS (
    SELECT DISTINCT ON (rider_id)
        contact_id,
        rider_id
    FROM rider_emergency_contacts
    ORDER BY rider_id, updated_at DESC, contact_id DESC
)
UPDATE rider_emergency_contacts rec
SET is_primary = TRUE,
    updated_at = NOW()
FROM candidates c
WHERE rec.contact_id = c.contact_id
  AND NOT EXISTS (
      SELECT 1
      FROM rider_emergency_contacts existing
      WHERE existing.rider_id = c.rider_id
        AND existing.is_primary = TRUE
  );

CREATE UNIQUE INDEX IF NOT EXISTS uq_rider_emergency_contacts_single_primary
    ON rider_emergency_contacts (rider_id)
    WHERE is_primary = TRUE;
