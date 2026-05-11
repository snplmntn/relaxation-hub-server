-- Add client account action statuses and migrate legacy bans to blocked.

ALTER TABLE users DROP CONSTRAINT IF EXISTS check_account_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_account_status_check;

UPDATE users
SET account_status = 'blocked',
    updated_at = CURRENT_TIMESTAMP
WHERE account_status = 'banned';

UPDATE users u
SET account_status = 'blocked',
    updated_at = CURRENT_TIMESTAMP
FROM moderation_blocks mb
WHERE u.user_id = mb.blocked_user_id
  AND mb.removed_at IS NULL;

ALTER TABLE users
ADD CONSTRAINT check_account_status
CHECK (account_status IN ('active', 'inactive', 'suspended', 'blocked', 'vip'));

CREATE INDEX IF NOT EXISTS idx_users_vip_status
ON users(account_status)
WHERE account_status = 'vip';
