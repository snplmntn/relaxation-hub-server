-- ============================================================================
-- Enforce uniqueness of users.primary_phone.
--
-- Historically primary_phone had no constraint (only primary_email did), so
-- admins could unknowingly create multiple clients sharing a phone number.
-- This adds a partial unique index mirroring idx_users_primary_email, covering
-- only live rows that actually have a phone value, so NULL/empty phones are
-- unaffected and multiple "no phone" users remain valid.
--
-- If this migration fails with a unique_violation, pre-existing duplicates must
-- be resolved first. Find them with:
--   SELECT primary_phone, COUNT(*), array_agg(user_id)
--   FROM users
--   WHERE deleted_at IS NULL AND primary_phone IS NOT NULL AND primary_phone <> ''
--   GROUP BY primary_phone HAVING COUNT(*) > 1;
-- ============================================================================

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_primary_phone_unique
ON users (primary_phone)
WHERE deleted_at IS NULL AND primary_phone IS NOT NULL AND primary_phone <> '';
