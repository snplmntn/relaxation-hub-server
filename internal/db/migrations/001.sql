-- ============================================================================
-- CONSOLIDATED MIGRATION: Initial Schema + Notification Preferences + Rate Limiting
-- ============================================================================

-- ============================================================================
-- 1. CORE USER & IDENTITY SCHEMA
-- ============================================================================

-- Stores the core user profile, independent of login method
-- Soft deletion enabled via deleted_at
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    full_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('client', 'therapist', 'admin')),
    
    -- These are for contact/display, NOT auth.
    -- They are set *after* an identity is verified.
    -- primary_phone: nullable and NOT UNIQUE to allow multiple users without phone
    primary_email VARCHAR(100) UNIQUE,
    primary_phone VARCHAR(20),
    
    profile_photo TEXT,
    gender VARCHAR(20),
    emergency_contact_name VARCHAR(100),
    emergency_contact_phone VARCHAR(20),
    
    -- Notification preferences (JSONB for flexibility)
    notification_preferences JSONB DEFAULT '{
      "push_notifications": true,
      "email_notifications": true,
      "sms_notifications": false,
      "booking_updates": true,
      "promotions": true,
      "rating_requests": true
    }'::jsonb,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for users
CREATE INDEX idx_users_role ON users(role) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_primary_email ON users(primary_email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

-- Stores all login methods linked to a user
CREATE TABLE user_auth_identities (
    identity_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    -- e.g., 'email', 'phone', 'google.com', 'apple.com'
    provider VARCHAR(30) NOT NULL,
    
    -- The unique ID for that provider:
    -- - 'email': the user's email address
    -- - 'phone': the user's phone number (e.g., +63917...)
    -- - 'google.com': the unique 'sub' ID from Google
    -- - 'apple.com': the unique 'sub' ID from Apple
    provider_key TEXT NOT NULL,
    
    -- Only used if provider is 'email'
    password_hash TEXT,
    
    is_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensures one user can't have two 'google.com' identities,
    -- and two users can't share the same 'email' login.
    UNIQUE (provider, provider_key)
);

-- Indexes for auth identities
CREATE INDEX idx_auth_identities_user ON user_auth_identities(user_id);
CREATE INDEX idx_auth_identities_provider_key ON user_auth_identities(provider, provider_key);

-- Stores user addresses (no longer has circular dependency)
CREATE TABLE addresses (
    address_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    label VARCHAR(50), -- e.g., 'Home', 'Work'
    street_address VARCHAR(150) NOT NULL,
    barangay VARCHAR(100),
    city VARCHAR(100) NOT NULL,
    province VARCHAR(100),
    postal_code VARCHAR(20),
    landmark VARCHAR(200),
    country VARCHAR(100) DEFAULT 'Philippines',
    
    -- NUMERIC(9,6) = 6 decimal places ≈ 11cm accuracy
    -- Philippines coordinates: lat ~5-20, lon ~116-127
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    
    is_default BOOLEAN DEFAULT FALSE,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for addresses
CREATE INDEX idx_addresses_user ON addresses(user_id) WHERE deleted_at IS NULL;
-- Ensure only one default address per user
CREATE UNIQUE INDEX idx_one_default_address ON addresses(user_id) 
    WHERE is_default = TRUE AND deleted_at IS NULL;

-- Caches ratings for clients
CREATE TABLE client_profiles (
    client_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    avg_rating NUMERIC(3,2) DEFAULT 0.0,
    total_reviews INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for client profiles
CREATE INDEX idx_client_profiles_rating ON client_profiles(avg_rating);

-- For user blocking
CREATE TABLE user_blocks (
    blocker_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    blocked_user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (blocker_user_id, blocked_user_id),
    
    -- Prevent self-blocking
    CHECK (blocker_user_id != blocked_user_id)
);

-- Index for user blocks
CREATE INDEX idx_user_blocks_blocker ON user_blocks(blocker_user_id);
CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_user_id);

-- ============================================================================
-- 2. THERAPIST & SERVICE CATALOG SCHEMA
-- ============================================================================

-- Optional: For grouping therapists by a physical location/partner
CREATE TABLE branches (
    branch_id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    address VARCHAR(255),
    phone VARCHAR(20),
    email VARCHAR(100),
    operating_hours JSONB,
    is_active BOOLEAN DEFAULT TRUE,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for branches
CREATE INDEX idx_branches_active ON branches(is_active) WHERE deleted_at IS NULL;

-- Defines a service (e.g., "Swedish Massage")
CREATE TABLE services (
    service_id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    preview_image_url TEXT,
    base_price NUMERIC(10,2) NOT NULL,
    duration_minutes INT NOT NULL DEFAULT 60,
    is_active BOOLEAN DEFAULT TRUE,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure positive values
    CHECK (base_price >= 0),
    CHECK (duration_minutes > 0)
);

-- Index for services
CREATE INDEX idx_services_active ON services(service_id) WHERE deleted_at IS NULL;

-- Therapist profiles (extension of users table)
CREATE TABLE therapist_profiles (
    therapist_id INT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    branch_id INT REFERENCES branches(branch_id) ON DELETE SET NULL,
    
    bio TEXT,
    years_experience INT,
    avg_rating NUMERIC(3,2) DEFAULT 0.0,
    total_reviews INT DEFAULT 0,
    is_available BOOLEAN DEFAULT TRUE,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure non-negative values
    CHECK (years_experience >= 0)
);

-- Indexes for therapist profiles
-- CRITICAL: For finding available therapists
CREATE INDEX idx_therapist_profiles_available ON therapist_profiles(is_available) 
    WHERE is_available = TRUE AND deleted_at IS NULL;
CREATE INDEX idx_therapist_profiles_rating ON therapist_profiles(avg_rating);
CREATE INDEX idx_therapist_profiles_branch ON therapist_profiles(branch_id) 
    WHERE deleted_at IS NULL;

-- Manages therapist certifications (replaces text array)
CREATE TABLE therapist_documents (
    document_id SERIAL PRIMARY KEY,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    
    document_url TEXT NOT NULL,
    document_type VARCHAR(50) CHECK (document_type IN ('Certification', 'ID', 'License')),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected')),
    
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for therapist documents
CREATE INDEX idx_therapist_documents_therapist ON therapist_documents(therapist_id);
CREATE INDEX idx_therapist_documents_status ON therapist_documents(status);

-- Many-to-many join table for therapist services
CREATE TABLE therapist_services (
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE RESTRICT,
    PRIMARY KEY (therapist_id, service_id)
);

-- Index for therapist services
-- CRITICAL: For finding therapists who offer a specific service
CREATE INDEX idx_therapist_services_service ON therapist_services(service_id);

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
        CHECK (status IN ('pending', 'confirmed', 'in_progress', 'completed', 'cancelled')),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
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
    WHERE status IN ('pending', 'confirmed');

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
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for reviews
CREATE INDEX idx_reviews_therapist ON reviews(therapist_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_client ON reviews(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_booking ON reviews(booking_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_created_at ON reviews(created_at DESC);

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
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
    
    type VARCHAR(50) NOT NULL, -- 'booking_confirmed', 'therapist_arrived', etc.
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

