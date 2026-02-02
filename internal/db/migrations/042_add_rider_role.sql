-- Add 'rider' role to users table check constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check 
    CHECK (role IN ('client', 'therapist', 'admin', 'super_admin', 'rider'));
