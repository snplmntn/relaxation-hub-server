-- Migration: Add at_branch tracking for therapist location status
-- This enables the "Return to Branch" check-in feature

ALTER TABLE therapist_profiles 
ADD COLUMN IF NOT EXISTS at_branch BOOLEAN DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS last_location_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create index for efficient filtering in matching queries
CREATE INDEX IF NOT EXISTS idx_therapist_profiles_at_branch 
ON therapist_profiles(at_branch) 
WHERE deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN therapist_profiles.at_branch IS 'True if therapist is at their assigned branch. False when assigned to a booking. Set to true via check-in button.';
COMMENT ON COLUMN therapist_profiles.last_location_update IS 'Timestamp of last location status change (at_branch toggle)';
