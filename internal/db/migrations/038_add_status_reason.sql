-- Add status_reason column to users table for ban/suspension audit trail
ALTER TABLE users ADD COLUMN status_reason TEXT;

-- Add index for users with non-active status (for admin queries)
CREATE INDEX idx_users_non_active_status ON users(account_status) WHERE account_status != 'active';
