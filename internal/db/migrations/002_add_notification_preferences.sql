-- ============================================================================
-- MIGRATION: Add Notification Preferences & Rate Limiting Support
-- ============================================================================
-- +migrate Up

-- Add notification_preferences column to users table
ALTER TABLE users
ADD COLUMN notification_preferences JSONB DEFAULT '{
  "push_notifications": true,
  "email_notifications": true,
  "sms_notifications": false,
  "booking_updates": true,
  "promotions": true,
  "rating_requests": true
}'::jsonb;

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

-- Index for rate limit lookups
CREATE INDEX idx_auth_rate_limits_identifier ON auth_rate_limits(identifier);
CREATE INDEX idx_auth_rate_limits_locked_until ON auth_rate_limits(locked_until);

-- Add updated_at trigger to users table if it doesn't exist
CREATE TRIGGER update_users_notification_prefs_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +migrate Down

-- Remove rate limit table
DROP INDEX IF EXISTS idx_auth_rate_limits_locked_until;
DROP INDEX IF EXISTS idx_auth_rate_limits_identifier;
DROP TABLE IF EXISTS auth_rate_limits;

-- Remove notification_preferences column
ALTER TABLE users
DROP COLUMN IF EXISTS notification_preferences;
