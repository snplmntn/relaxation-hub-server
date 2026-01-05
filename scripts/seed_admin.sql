-- ============================================================================
-- SEED SCRIPT: Create Admin Account
-- ============================================================================
-- Run this script to create an admin user with password authentication.
-- 
-- Usage:
--   psql -U <username> -d <database> -f seed_admin.sql
--
-- Default Credentials:
--   Email: admin@relaxationhub.com
--   Password: admin123
--
-- IMPORTANT: Change the password immediately after first login in production!
-- ============================================================================

-- Password hash for 'admin123' using bcrypt
-- You can generate a new hash using: https://bcrypt-generator.com/ or via your backend
-- This hash is: $2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X.VymGI6.UOeYoGKW

DO $$
DECLARE
    v_user_id INT;
    v_admin_email VARCHAR := 'admin@relaxationhub.com';
    -- bcrypt hash for 'admin123' (cost factor 12)
    v_password_hash VARCHAR := '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X.VymGI6.UOeYoGKW';
BEGIN
    -- Check if admin already exists
    IF EXISTS (SELECT 1 FROM users WHERE primary_email = v_admin_email AND deleted_at IS NULL) THEN
        RAISE NOTICE 'Admin user with email % already exists. Skipping creation.', v_admin_email;
        RETURN;
    END IF;

    -- Create the admin user
    INSERT INTO users (
        full_name,
        role,
        primary_email,
        account_status,
        created_at,
        updated_at
    ) VALUES (
        'System Administrator',
        'admin',
        v_admin_email,
        'active',
        CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP
    ) RETURNING user_id INTO v_user_id;

    -- Create the auth identity with password
    INSERT INTO user_auth_identities (
        user_id,
        provider,
        provider_key,
        password_hash,
        is_verified,
        created_at
    ) VALUES (
        v_user_id,
        'email',
        v_admin_email,
        v_password_hash,
        TRUE,
        CURRENT_TIMESTAMP
    );

    RAISE NOTICE 'Admin user created successfully!';
    RAISE NOTICE '  User ID: %', v_user_id;
    RAISE NOTICE '  Email: %', v_admin_email;
    RAISE NOTICE '  Password: admin123';
    RAISE NOTICE '';
    RAISE NOTICE 'IMPORTANT: Change this password immediately in production!';

END $$;

-- Verify the admin was created
SELECT 
    u.user_id,
    u.full_name,
    u.role,
    u.primary_email,
    u.account_status,
    uai.provider,
    uai.is_verified
FROM users u
JOIN user_auth_identities uai ON u.user_id = uai.user_id
WHERE u.role IN ('admin', 'super_admin')
  AND u.deleted_at IS NULL;
