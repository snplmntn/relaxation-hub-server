-- Migration: add accept_assignments toggle to therapists
-- Adds a boolean column `accept_assignments` defaulting to true

-- The project stores therapist rows in `therapist_profiles`.
ALTER TABLE therapist_profiles
    ADD COLUMN IF NOT EXISTS accept_assignments BOOLEAN NOT NULL DEFAULT TRUE;

-- Optional index if you need to query by this flag frequently
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_accept_assignments ON therapist_profiles(accept_assignments);
