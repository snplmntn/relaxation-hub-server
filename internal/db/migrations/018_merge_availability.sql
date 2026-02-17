-- Merge is_available into accept_assignments and drop is_available
UPDATE therapist_profiles 
SET accept_assignments = (accept_assignments AND is_available);

ALTER TABLE therapist_profiles DROP COLUMN is_available;
