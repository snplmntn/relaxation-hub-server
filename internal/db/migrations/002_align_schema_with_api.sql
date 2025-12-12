-- ============================================================================
-- MIGRATION 002: Align Database Schema with API Expectations
-- ============================================================================
-- This migration updates the schema to match the API handlers and documentation
-- Run this after 001.sql has been applied

-- ============================================================================
-- 1. UPDATE ADDRESSES TABLE
-- ============================================================================

-- Rename street to street_address
ALTER TABLE addresses RENAME COLUMN street TO street_address;

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
ALTER TABLE branches RENAME COLUMN phone_number TO phone;

-- Add missing columns
ALTER TABLE branches ADD COLUMN IF NOT EXISTS operating_hours JSONB;
ALTER TABLE branches ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 3. UPDATE SERVICES TABLE
-- ============================================================================

-- Rename min_duration_minutes to duration_minutes
ALTER TABLE services RENAME COLUMN min_duration_minutes TO duration_minutes;

-- Add missing columns
ALTER TABLE services ADD COLUMN IF NOT EXISTS category VARCHAR(50);
ALTER TABLE services ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
ALTER TABLE services ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- ============================================================================
-- 4. UPDATE PROMOTIONS TABLE
-- ============================================================================

-- Rename discount_percent to discount_percentage
ALTER TABLE promotions RENAME COLUMN discount_percent TO discount_percentage;

-- Rename usage_limit to max_uses
ALTER TABLE promotions RENAME COLUMN usage_limit TO max_uses;

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
CREATE TRIGGER update_addresses_updated_at 
    BEFORE UPDATE ON addresses 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_branches_updated_at ON branches;
CREATE TRIGGER update_branches_updated_at 
    BEFORE UPDATE ON branches 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_services_updated_at ON services;
CREATE TRIGGER update_services_updated_at 
    BEFORE UPDATE ON services 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_promotions_updated_at ON promotions;
CREATE TRIGGER update_promotions_updated_at 
    BEFORE UPDATE ON promotions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
CREATE TRIGGER update_notifications_updated_at 
    BEFORE UPDATE ON notifications 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_therapist_profiles_updated_at ON therapist_profiles;
CREATE TRIGGER update_therapist_profiles_updated_at 
    BEFORE UPDATE ON therapist_profiles 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- MIGRATION COMPLETE
-- ============================================================================
-- Schema is now aligned with API expectations
-- All tables have consistent naming and required fields
