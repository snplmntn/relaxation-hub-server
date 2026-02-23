-- ============================================================================
-- DATABASE SEED SCRIPT: Services & Users
-- ============================================================================

-- 1. Insert Services
-- Note: The "promo" rate is handled via the existing 'WELCOME50' promotion 
-- which gives exactly 50 PHP off the base price for all services.
INSERT INTO services (name, description, base_price, duration_minutes, category, therapist_commission, is_active)
VALUES 
    ('Swedish Massage', 'A classic full-body massage using gentle to firm pressure.', 450.00, 60, 'massage', 130.00, TRUE),
    ('Signature Massage', 'Our specialized signature massage combining multiple techniques.', 450.00, 60, 'massage', 130.00, TRUE),
    ('Thai Massage', 'An active massage combining acupressure and assisted yoga postures.', 570.00, 60, 'massage', 180.00, TRUE),
    ('Shiatsu Massage', 'A traditional Japanese massage technique using finger pressure.', 570.00, 60, 'massage', 180.00, TRUE),
    ('Foot Reflexology', 'Targeted pressure point massage on the feet to promote overall wellness.', 570.00, 60, 'massage', 180.00, TRUE),
    ('Foot Massage', 'A relaxing massage focusing purely on the lower legs and feet.', 570.00, 60, 'massage', 180.00, TRUE),
    ('Pre-natal Massage', 'A gentle massage specially designed for expectant mothers.', 570.00, 60, 'massage', 180.00, TRUE),
    ('Hilot', 'A traditional Filipino healing massage focused on removing energy blockages.', 570.00, 60, 'massage', 180.00, TRUE)
ON CONFLICT DO NOTHING;

-- 2. Insert Users (Admin, Client, Therapist, Rider)
DO $$
DECLARE
    new_admin_id INT;
    new_client_id INT;
    new_therapist_id INT;
    new_rider_id INT;
    password_hash VARCHAR := '$2a$10$u7KbWvzUiJVd99UGgYQwgewyD9MkeWRyaH3PziOzi9N51FxJAe4h2'; -- 'Sean1234!'
BEGIN
    -- [A] Admin User
    IF NOT EXISTS (SELECT 1 FROM users WHERE primary_email = 'admin@gmail.com') THEN
        INSERT INTO users (full_name, role, primary_email, created_at, updated_at) 
        VALUES ('Super Admin', 'admin', 'admin@gmail.com', NOW(), NOW())
        RETURNING user_id INTO new_admin_id;

        INSERT INTO user_auth_identities (user_id, provider, provider_key, password_hash, is_verified, created_at)
        VALUES (new_admin_id, 'email', 'admin@gmail.com', password_hash, TRUE, NOW());
    END IF;

    -- [B] Client User (Sean)
    IF NOT EXISTS (SELECT 1 FROM users WHERE primary_email = 'sean@gmail.com') THEN
        INSERT INTO users (full_name, role, primary_email, created_at, updated_at) 
        VALUES ('Sean Client', 'client', 'sean@gmail.com', NOW(), NOW())
        RETURNING user_id INTO new_client_id;

        INSERT INTO user_auth_identities (user_id, provider, provider_key, password_hash, is_verified, created_at)
        VALUES (new_client_id, 'email', 'sean@gmail.com', password_hash, TRUE, NOW());

        INSERT INTO addresses (user_id, label, street_address, city, country, is_default, created_at, updated_at)
        VALUES (new_client_id, 'Home', '123 Client St.', 'Manila', 'Philippines', TRUE, NOW(), NOW());
    END IF;

    -- [C] Therapist User
    IF NOT EXISTS (SELECT 1 FROM users WHERE primary_email = 'therapist@gmail.com') THEN
        INSERT INTO users (full_name, role, primary_email, created_at, updated_at) 
        VALUES ('Test Therapist', 'therapist', 'therapist@gmail.com', NOW(), NOW())
        RETURNING user_id INTO new_therapist_id;

        INSERT INTO user_auth_identities (user_id, provider, provider_key, password_hash, is_verified, created_at)
        VALUES (new_therapist_id, 'email', 'therapist@gmail.com', password_hash, TRUE, NOW());

        -- Note: The trigger automatically inserts into therapist_wallets in migration 051
        INSERT INTO therapist_profiles (therapist_id, is_verified, accept_assignments, avg_rating, total_reviews, total_bookings, created_at, updated_at)
        VALUES (new_therapist_id, TRUE, TRUE, 5.0, 0, 0, NOW(), NOW());

        INSERT INTO addresses (user_id, label, street_address, city, country, is_default, created_at, updated_at)
        VALUES (new_therapist_id, 'Home Base', '456 Therapist Blvd.', 'Makati', 'Philippines', TRUE, NOW(), NOW());
    END IF;

    -- [D] Rider User
    IF NOT EXISTS (SELECT 1 FROM users WHERE primary_email = 'rider@gmail.com') THEN
        INSERT INTO users (full_name, role, primary_email, created_at, updated_at) 
        VALUES ('Test Rider', 'rider', 'rider@gmail.com', NOW(), NOW())
        RETURNING user_id INTO new_rider_id;

        INSERT INTO user_auth_identities (user_id, provider, provider_key, password_hash, is_verified, created_at)
        VALUES (new_rider_id, 'email', 'rider@gmail.com', password_hash, TRUE, NOW());

        INSERT INTO rider_profiles (user_id, vehicle_type, license_plate, license_number, is_online, rating, total_trips, created_at, updated_at)
        VALUES (new_rider_id, 'motorcycle', 'XYZ-1234', 'A01-12-123456', TRUE, 5.0, 0, NOW(), NOW());

        INSERT INTO addresses (user_id, label, street_address, city, country, is_default, created_at, updated_at)
        VALUES (new_rider_id, 'Home Base', '789 Rider Ave.', 'Taguig', 'Philippines', TRUE, NOW(), NOW());
    END IF;

END $$;
