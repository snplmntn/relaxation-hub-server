-- Add client blocked account status.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_account_status_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_account_status;

ALTER TABLE users
ADD CONSTRAINT check_account_status
CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive', 'blocked'));
