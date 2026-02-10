-- Migration 045: Fix Therapist Services Cascade Deletion
-- Changes the foreign key constraint on therapist_services(service_id) to ON DELETE CASCADE
-- This allows deleting a service to automatically remove it from all therapists' profiles

ALTER TABLE therapist_services
DROP CONSTRAINT IF EXISTS therapist_services_service_id_fkey;

ALTER TABLE therapist_services
ADD CONSTRAINT therapist_services_service_id_fkey
    FOREIGN KEY (service_id)
    REFERENCES services(service_id)
    ON DELETE CASCADE;
