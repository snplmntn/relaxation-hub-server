-- Add VIP flag for clients.

ALTER TABLE users
ADD COLUMN IF NOT EXISTS is_vip BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_vip ON users(is_vip) WHERE is_vip = TRUE;
