-- Migration 006: add is_verified to therapist_profiles
ALTER TABLE therapist_profiles
    ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;

-- Ensure non-null values
UPDATE therapist_profiles SET is_verified = FALSE WHERE is_verified IS NULL;

-- Optional index for queries filtering by verified status
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_verified ON therapist_profiles(is_verified) WHERE deleted_at IS NULL;

-- end
