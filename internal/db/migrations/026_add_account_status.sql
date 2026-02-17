-- Add account_status column to users table
ALTER TABLE users ADD COLUMN account_status VARCHAR(20) NOT NULL DEFAULT 'active';

-- Add check constraint for valid statuses
ALTER TABLE users ADD CONSTRAINT check_account_status CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive'));

-- Add index for account_status
CREATE INDEX idx_users_account_status ON users(account_status);
