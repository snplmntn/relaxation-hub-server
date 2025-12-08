-- ============================================================================
-- CONSOLIDATED MIGRATION: Initial Schema + Hardening + Phone Constraint Fix
-- ============================================================================
-- +migrate Up

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
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensures one user can't have two 'google.com' identities,
    -- and two users can't share the same 'email' login.
    UNIQUE (provider, provider_key),
    
    -- Prevent duplicate provider per user
    UNIQUE (user_id, provider)
);

-- Indexes for auth identities
CREATE INDEX idx_auth_identities_user ON user_auth_identities(user_id);
CREATE INDEX idx_auth_identities_provider_key ON user_auth_identities(provider, provider_key);

-- Stores user addresses
CREATE TABLE addresses (
    address_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE RESTRICT,
    
    label VARCHAR(50), -- e.g., 'Home', 'Work'
    street VARCHAR(150) NOT NULL,
    city VARCHAR(100) NOT NULL,
    province VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100) DEFAULT 'Philippines',
    
    -- NUMERIC(9,6) = 6 decimal places ≈ 11cm accuracy
    -- Philippines coordinates: lat ~5-20, lon ~116-127
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    
    is_default BOOLEAN DEFAULT FALSE,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Geospatial bounds for Philippines
    CHECK (latitude IS NULL OR (latitude >= 5 AND latitude <= 20)),
    CHECK (longitude IS NULL OR (longitude >= 116 AND longitude <= 127))
);

-- Indexes for addresses
CREATE INDEX idx_addresses_user ON addresses(user_id) WHERE deleted_at IS NULL;
-- Ensure only one default address per user
CREATE UNIQUE INDEX idx_one_default_address ON addresses(user_id) 
    WHERE is_default = TRUE AND deleted_at IS NULL;

-- Create trigger function for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

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
    phone_number VARCHAR(20),
    email VARCHAR(100),
    address_id INT REFERENCES addresses(address_id) ON DELETE RESTRICT,
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
    base_price NUMERIC(10,2) NOT NULL,
    min_duration_minutes INT NOT NULL DEFAULT 60,
    
    -- Soft deletion
    deleted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure positive values
    CHECK (base_price >= 0),
    CHECK (min_duration_minutes > 0)
);

-- Index for services
CREATE INDEX idx_services_active ON services(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_not_deleted ON services(service_id) WHERE deleted_at IS NULL;

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
CREATE INDEX idx_therapist_profiles_rating_active ON therapist_profiles(avg_rating) 
    WHERE deleted_at IS NULL;
CREATE INDEX idx_therapist_profiles_branch ON therapist_profiles(branch_id) 
    WHERE deleted_at IS NULL;

-- Manages therapist certifications (replaces text array)
CREATE TABLE therapist_documents (
    document_id SERIAL PRIMARY KEY,
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    
    document_url TEXT NOT NULL,
    document_type VARCHAR(50) CHECK (document_type IN ('Certification', 'ID', 'License')),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected')),
    
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for therapist documents
CREATE INDEX idx_therapist_documents_therapist ON therapist_documents(therapist_id);
CREATE INDEX idx_therapist_documents_status ON therapist_documents(status);

-- Many-to-many join table for therapist services
CREATE TABLE therapist_services (
    therapist_id INT REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    service_id INT REFERENCES services(service_id) ON DELETE RESTRICT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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
    discount_percent INT CHECK (discount_percent BETWEEN 1 AND 100),
    
    valid_from TIMESTAMP,
    valid_until TIMESTAMP,
    usage_limit INT DEFAULT 1,
    
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
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Data validation constraints
    CHECK (duration_minutes > 0),
    CHECK (final_total >= 0),
    CHECK (actual_end IS NULL OR actual_start IS NULL OR actual_end > actual_start),
    CHECK (discount >= 0),
    CHECK (raw_total >= 0),
    CHECK (raw_total IS NULL OR final_total IS NULL OR final_total = raw_total - discount),
    CHECK (raw_total IS NULL OR discount <= raw_total)
);

-- Indexes for bookings
CREATE INDEX idx_bookings_client ON bookings(client_id) WHERE status != 'cancelled' AND deleted_at IS NULL;
CREATE INDEX idx_bookings_therapist ON bookings(therapist_id) WHERE status != 'cancelled' AND deleted_at IS NULL;
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_scheduled ON bookings(scheduled_start) WHERE status IN ('pending', 'confirmed');

-- Payments table
CREATE TABLE payments (
    payment_id SERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    amount NUMERIC(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'failed', 'refunded')),
    
    transaction_id VARCHAR(255),
    payment_method VARCHAR(50),
    
    paid_at TIMESTAMP,
    refunded_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Data integrity constraints
    CHECK (status <> 'paid' OR paid_at IS NOT NULL),
    CHECK (status <> 'paid' OR transaction_id IS NOT NULL),
    CHECK (status <> 'refunded' OR refunded_at IS NOT NULL)
);

-- Indexes for payments
CREATE INDEX idx_payments_booking ON payments(booking_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_transaction ON payments(transaction_id);

-- Reviews from clients about therapists
CREATE TABLE reviews (
    review_id SERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    client_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for reviews
CREATE INDEX idx_reviews_therapist ON reviews(therapist_id);
CREATE INDEX idx_reviews_client ON reviews(client_id);
CREATE INDEX idx_reviews_booking ON reviews(booking_id);

-- Reviews from therapists about clients
CREATE TABLE client_reviews (
    review_id SERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    therapist_id INT NOT NULL REFERENCES therapist_profiles(therapist_id) ON DELETE RESTRICT,
    client_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for client reviews
CREATE INDEX idx_client_reviews_therapist ON client_reviews(therapist_id);
CREATE INDEX idx_client_reviews_client ON client_reviews(client_id);
CREATE INDEX idx_client_reviews_booking ON client_reviews(booking_id);

-- ============================================================================
-- 4. MESSAGING & NOTIFICATIONS
-- ============================================================================

-- Conversations between users
CREATE TABLE conversations (
    conversation_id SERIAL PRIMARY KEY,
    user_a_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    user_b_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Prevent self-conversations
    CHECK (user_a_id != user_b_id),
    -- Prevent duplicate conversations
    UNIQUE (LEAST(user_a_id, user_b_id), GREATEST(user_a_id, user_b_id))
);

-- Indexes for conversations
CREATE INDEX idx_conversations_user_a ON conversations(user_a_id);
CREATE INDEX idx_conversations_user_b ON conversations(user_b_id);

-- Messages in conversations
CREATE TABLE messages (
    message_id SERIAL PRIMARY KEY,
    conversation_id INT NOT NULL REFERENCES conversations(conversation_id) ON DELETE RESTRICT,
    sender_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    message_type VARCHAR(20) DEFAULT 'text' CHECK (message_type IN ('text', 'image', 'system')),
    content TEXT,
    
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for messages
CREATE INDEX idx_messages_conversation ON messages(conversation_id);
CREATE INDEX idx_messages_sender ON messages(sender_id);
CREATE INDEX idx_messages_is_read ON messages(is_read);

-- Emergency alerts (SOS)
CREATE TABLE emergency_alerts (
    alert_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'resolved', 'cancelled')),
    
    latitude NUMERIC(9,6) NOT NULL,
    longitude NUMERIC(9,6) NOT NULL,
    description TEXT,
    
    responder_id INT REFERENCES users(user_id) ON DELETE SET NULL,
    resolved_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Geospatial bounds for Philippines
    CHECK (latitude >= 5 AND latitude <= 20),
    CHECK (longitude >= 116 AND longitude <= 127)
);

-- Indexes for emergency alerts
CREATE INDEX idx_emergency_alerts_user ON emergency_alerts(user_id);
CREATE INDEX idx_emergency_alerts_status ON emergency_alerts(status);
CREATE INDEX idx_emergency_alerts_created ON emergency_alerts(created_at);

-- Notifications
CREATE TABLE notifications (
    notification_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    
    type VARCHAR(50) NOT NULL, -- e.g., 'booking_confirmed', 'review_received'
    title VARCHAR(255) NOT NULL,
    message TEXT,
    
    related_booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    related_user_id INT REFERENCES users(user_id) ON DELETE SET NULL,
    
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for notifications
CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_is_read ON notifications(is_read);
CREATE INDEX idx_notifications_created ON notifications(created_at);

-- Live tracking
CREATE TABLE live_locations (
    location_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL,
    
    latitude NUMERIC(9,6) NOT NULL,
    longitude NUMERIC(9,6) NOT NULL,
    
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Geospatial bounds for Philippines
    CHECK (latitude >= 5 AND latitude <= 20),
    CHECK (longitude >= 116 AND longitude <= 127)
);

-- Indexes for live locations
CREATE INDEX idx_live_locations_user ON live_locations(user_id);
CREATE INDEX idx_live_locations_booking ON live_locations(booking_id);
CREATE INDEX idx_live_locations_updated ON live_locations(updated_at);

-- ============================================================================
-- 5. TRIGGERS FOR AUDIT COLUMNS
-- ============================================================================

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

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

CREATE TRIGGER update_therapist_profiles_updated_at
    BEFORE UPDATE ON therapist_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_therapist_documents_updated_at
    BEFORE UPDATE ON therapist_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_therapist_services_updated_at
    BEFORE UPDATE ON therapist_services
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_promotions_updated_at
    BEFORE UPDATE ON promotions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_bookings_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payments_updated_at
    BEFORE UPDATE ON payments
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

CREATE TRIGGER update_emergency_alerts_updated_at
    BEFORE UPDATE ON emergency_alerts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_auth_identities_updated_at
    BEFORE UPDATE ON user_auth_identities
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +migrate Down

DROP TRIGGER IF EXISTS update_user_auth_identities_updated_at ON user_auth_identities;
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
DROP TRIGGER IF EXISTS update_emergency_alerts_updated_at ON emergency_alerts;
DROP TRIGGER IF EXISTS update_conversations_updated_at ON conversations;
DROP TRIGGER IF EXISTS update_client_reviews_updated_at ON client_reviews;
DROP TRIGGER IF EXISTS update_reviews_updated_at ON reviews;
DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;
DROP TRIGGER IF EXISTS update_bookings_updated_at ON bookings;
DROP TRIGGER IF EXISTS update_promotions_updated_at ON promotions;
DROP TRIGGER IF EXISTS update_therapist_services_updated_at ON therapist_services;
DROP TRIGGER IF EXISTS update_therapist_documents_updated_at ON therapist_documents;
DROP TRIGGER IF EXISTS update_therapist_profiles_updated_at ON therapist_profiles;
DROP TRIGGER IF EXISTS update_services_updated_at ON services;
DROP TRIGGER IF EXISTS update_branches_updated_at ON branches;
DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

DROP TABLE IF EXISTS live_locations CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS emergency_alerts CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
DROP TABLE IF EXISTS client_reviews CASCADE;
DROP TABLE IF EXISTS reviews CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS bookings CASCADE;
DROP TABLE IF EXISTS user_promotions CASCADE;
DROP TABLE IF EXISTS promotions CASCADE;
DROP TABLE IF EXISTS therapist_services CASCADE;
DROP TABLE IF EXISTS therapist_documents CASCADE;
DROP TABLE IF EXISTS therapist_profiles CASCADE;
DROP TABLE IF EXISTS services CASCADE;
DROP TABLE IF EXISTS branches CASCADE;
DROP TABLE IF EXISTS user_blocks CASCADE;
DROP TABLE IF EXISTS client_profiles CASCADE;
DROP TABLE IF EXISTS addresses CASCADE;
DROP TABLE IF EXISTS user_auth_identities CASCADE;
DROP TABLE IF EXISTS users CASCADE;

DROP FUNCTION IF EXISTS update_updated_at_column();
