-- ============================================================================
-- Add an optional nickname for users.
-- Used by the admin day-view "Edit Staff" UI to give a therapist a short
-- display nickname alongside their full legal name.
-- ============================================================================

ALTER TABLE users
ADD COLUMN IF NOT EXISTS nickname VARCHAR(100);
