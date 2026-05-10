-- ============================================================================
-- Relaxation Hub Supabase seed script
-- Seeds demo users, Kamias branch, booking-ready address/service area,
-- services, and therapist service capabilities.
--
-- Seeded password for all accounts: Sean1234!
-- Run this file in Supabase SQL Editor as one script.
-- ============================================================================

BEGIN;

DO $$
DECLARE
  v_password_hash text := '$2a$10$u7KbWvzUiJVd99UGgYQwgewyD9MkeWRyaH3PziOzi9N51FxJAe4h2';
  v_admin_id bigint;
  v_therapist_id bigint;
  v_rider_user_id bigint;
  v_client_id bigint;
  v_branch_id bigint;
  v_address_id bigint;
  svc record;
  v_service_id bigint;
BEGIN
  -- Users
  INSERT INTO users (full_name, role, primary_email, primary_phone, account_status, deleted_at, updated_at)
  VALUES
    ('Seed Admin', 'admin', 'admin@relaxationhub.ph', '+639170000001', 'active', NULL, NOW()),
    ('Seed Therapist', 'therapist', 'therapist@relaxationhub.ph', '+639170000002', 'active', NULL, NOW()),
    ('Seed Rider', 'rider', 'rider@relaxationhub.ph', '+639170000003', 'active', NULL, NOW()),
    ('Seed Client', 'client', 'client@relaxationhub.ph', '+639170000004', 'active', NULL, NOW())
  ON CONFLICT (primary_email) DO UPDATE SET
    full_name = EXCLUDED.full_name,
    role = EXCLUDED.role,
    primary_phone = EXCLUDED.primary_phone,
    account_status = 'active',
    deleted_at = NULL,
    updated_at = NOW();

  SELECT user_id INTO v_admin_id FROM users WHERE primary_email = 'admin@relaxationhub.ph';
  SELECT user_id INTO v_therapist_id FROM users WHERE primary_email = 'therapist@relaxationhub.ph';
  SELECT user_id INTO v_rider_user_id FROM users WHERE primary_email = 'rider@relaxationhub.ph';
  SELECT user_id INTO v_client_id FROM users WHERE primary_email = 'client@relaxationhub.ph';

  -- Email/password auth identities
  INSERT INTO user_auth_identities (user_id, provider, provider_key, password_hash, is_verified, created_at)
  VALUES
    (v_admin_id, 'email', 'admin@relaxationhub.ph', v_password_hash, TRUE, NOW()),
    (v_therapist_id, 'email', 'therapist@relaxationhub.ph', v_password_hash, TRUE, NOW()),
    (v_rider_user_id, 'email', 'rider@relaxationhub.ph', v_password_hash, TRUE, NOW()),
    (v_client_id, 'email', 'client@relaxationhub.ph', v_password_hash, TRUE, NOW())
  ON CONFLICT (provider, provider_key) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    password_hash = EXCLUDED.password_hash,
    is_verified = TRUE;

  -- Kamias branch
  UPDATE branches
  SET
    branch_name = 'Kamias',
    address_line = 'Kamias Road, Kamias, Quezon City',
    barangay = 'Kamias',
    city = 'Quezon City',
    province = 'Metro Manila',
    postal_code = '1102',
    contact_no = '+639170000010',
    email = 'kamias@relaxationhub.ph',
    is_active = TRUE,
    deleted_at = NULL,
    updated_at = NOW()
  WHERE branch_name = 'Kamias'
    AND deleted_at IS NULL;

  SELECT branch_id INTO v_branch_id
  FROM branches
  WHERE branch_name = 'Kamias'
    AND deleted_at IS NULL
  ORDER BY branch_id
  LIMIT 1;

  IF v_branch_id IS NULL THEN
    INSERT INTO branches (
      branch_name, address_line, barangay, city, province, postal_code,
      contact_no, email, is_active, created_at, updated_at
    )
    VALUES (
      'Kamias',
      'Kamias Road, Kamias, Quezon City',
      'Kamias',
      'Quezon City',
      'Metro Manila',
      '1102',
      '+639170000010',
      'kamias@relaxationhub.ph',
      TRUE,
      NOW(),
      NOW()
    )
    RETURNING branch_id INTO v_branch_id;
  END IF;

  -- Client profile/address for booking readiness
  INSERT INTO client_profiles (client_id, avg_rating, total_reviews, created_at)
  VALUES (v_client_id, 0, 0, NOW())
  ON CONFLICT (client_id) DO NOTHING;

  UPDATE addresses
  SET is_default = FALSE, updated_at = NOW()
  WHERE user_id = v_client_id
    AND deleted_at IS NULL;

  UPDATE addresses
  SET
    label = 'Seed Kamias Address',
    street_address = 'Kamias Road, Kamias, Quezon City',
    barangay = 'Kamias',
    city = 'Quezon City',
    province = 'Metro Manila',
    postal_code = '1102',
    country = 'Philippines',
    is_default = TRUE,
    deleted_at = NULL,
    updated_at = NOW()
  WHERE user_id = v_client_id
    AND label = 'Seed Kamias Address';

  SELECT address_id INTO v_address_id
  FROM addresses
  WHERE user_id = v_client_id
    AND label = 'Seed Kamias Address'
    AND deleted_at IS NULL
  ORDER BY address_id
  LIMIT 1;

  IF v_address_id IS NULL THEN
    INSERT INTO addresses (
      user_id, label, street_address, barangay, city, province, postal_code,
      country, is_default, created_at, updated_at
    )
    VALUES (
      v_client_id,
      'Seed Kamias Address',
      'Kamias Road, Kamias, Quezon City',
      'Kamias',
      'Quezon City',
      'Metro Manila',
      '1102',
      'Philippines',
      TRUE,
      NOW(),
      NOW()
    );
  END IF;

  -- Service area coverage
  INSERT INTO service_areas (
    area_key, parent_code, name, level, status, min_booking_minutes, created_at, updated_at
  )
  VALUES (
    'barangay:quezon-city:kamias',
    'quezon-city',
    'Kamias, Quezon City',
    'barangay',
    'covered',
    0,
    NOW(),
    NOW()
  )
  ON CONFLICT (area_key) DO UPDATE SET
    parent_code = EXCLUDED.parent_code,
    name = EXCLUDED.name,
    level = EXCLUDED.level,
    status = 'covered',
    min_booking_minutes = 0,
    updated_at = NOW();

  -- Therapist profile
  INSERT INTO therapist_profiles (
    therapist_id, branch_id, bio, years_experience, avg_rating, total_reviews,
    total_bookings, accept_assignments, is_verified, at_branch, deleted_at,
    created_at, updated_at
  )
  VALUES (
    v_therapist_id,
    v_branch_id,
    'Seed therapist for Kamias booking validation.',
    5,
    5.00,
    0,
    0,
    TRUE,
    TRUE,
    TRUE,
    NULL,
    NOW(),
    NOW()
  )
  ON CONFLICT (therapist_id) DO UPDATE SET
    branch_id = EXCLUDED.branch_id,
    bio = EXCLUDED.bio,
    years_experience = EXCLUDED.years_experience,
    avg_rating = EXCLUDED.avg_rating,
    accept_assignments = TRUE,
    is_verified = TRUE,
    at_branch = TRUE,
    deleted_at = NULL,
    updated_at = NOW();

  -- Rider profile
  UPDATE rider_profiles
  SET
    vehicle_type = 'motorcycle',
    license_plate = 'SEED-1234',
    license_number = 'SEED-LICENSE-1234',
    is_online = TRUE,
    updated_at = NOW()
  WHERE user_id = v_rider_user_id;

  IF NOT FOUND THEN
    INSERT INTO rider_profiles (
      user_id, vehicle_type, license_plate, license_number, is_online,
      rating, total_trips, created_at, updated_at
    )
    VALUES (
      v_rider_user_id,
      'motorcycle',
      'SEED-1234',
      'SEED-LICENSE-1234',
      TRUE,
      5.00,
      0,
      NOW(),
      NOW()
    );
  END IF;

  -- Services and therapist capabilities
  FOR svc IN
    SELECT *
    FROM (
      VALUES
        ('swedish massage', 'swedish massage', 'A classic, full-body massage using long gliding strokes to promote relaxation, improve circulation, and relieve muscle tension.', 450.00, 60, 130.00),
        ('signature massage', 'signature massage', 'Our signature therapy blends multiple techniques tailored to your needs for a balanced combination of relaxation and targeted tension relief.', 450.00, 60, 130.00),
        ('thai massage', 'thai massage', 'An invigorating combination of stretching and rhythmic compression using assisted yoga-like positions to increase flexibility and energy flow.', 570.00, 60, 180.00),
        ('shiatsu massage', 'shiatsu massage', 'A Japanese technique using rhythmic finger pressure on key points to restore balance, reduce stress, and relieve pain.', 570.00, 60, 180.00),
        ('foot reflex', 'foot reflexology', 'Focused reflexology treatment stimulating pressure points on the feet to promote whole-body healing and deep relaxation.', 570.00, 60, 180.00),
        ('foot massage', 'foot massage', 'Therapeutic foot massage combining kneading and pressure to ease foot and lower-leg tension and improve circulation.', 570.00, 60, 180.00),
        ('pre-natal massage', 'pre-natal massage', 'Gentle, pregnancy-safe massage that relieves lower back pain, reduces swelling, and supports overall maternal comfort.', 570.00, 60, 180.00),
        ('hilot', 'hilot', 'A traditional Filipino healing massage using rhythmic movements and herbal compresses to relieve muscle pain and rebalance the body.', 570.00, 60, 180.00)
    ) AS s(name, alias_name, description, base_price, duration_minutes, therapist_commission)
  LOOP
    SELECT service_id INTO v_service_id
    FROM services
    WHERE lower(name) IN (lower(svc.name), lower(svc.alias_name))
      AND deleted_at IS NULL
    ORDER BY
      CASE WHEN lower(name) = lower(svc.name) THEN 0 ELSE 1 END,
      service_id
    LIMIT 1;

    IF v_service_id IS NULL THEN
      INSERT INTO services (
        name, description, base_price, duration_minutes, category,
        therapist_commission, is_active, deleted_at, created_at, updated_at
      )
      VALUES (
        svc.name, svc.description, svc.base_price, svc.duration_minutes,
        'massage', svc.therapist_commission, TRUE, NULL, NOW(), NOW()
      )
      RETURNING service_id INTO v_service_id;
    ELSE
      UPDATE services
      SET
        name = svc.name,
        description = svc.description,
        base_price = svc.base_price,
        duration_minutes = svc.duration_minutes,
        category = 'massage',
        therapist_commission = svc.therapist_commission,
        is_active = TRUE,
        deleted_at = NULL,
        updated_at = NOW()
      WHERE service_id = v_service_id;
    END IF;

    UPDATE services
    SET is_active = FALSE, deleted_at = NOW(), updated_at = NOW()
    WHERE service_id <> v_service_id
      AND lower(name) IN (lower(svc.name), lower(svc.alias_name))
      AND deleted_at IS NULL;

    INSERT INTO therapist_services (
      therapist_id, service_id, supports_soft, supports_moderate, supports_hard
    )
    VALUES (
      v_therapist_id, v_service_id, TRUE, TRUE, TRUE
    )
    ON CONFLICT (therapist_id, service_id) DO UPDATE SET
      supports_soft = TRUE,
      supports_moderate = TRUE,
      supports_hard = TRUE;
  END LOOP;
END $$;

COMMIT;

-- Verification output: only show every therapist service setting seeded above.

SELECT
  u.primary_email AS therapist_email,
  s.name AS service_name,
  ts.*
FROM therapist_services ts
JOIN users u ON u.user_id = ts.therapist_id
JOIN services s ON s.service_id = ts.service_id
WHERE u.primary_email = 'therapist@relaxationhub.ph'
  AND lower(s.name) IN (
    'swedish massage',
    'signature massage',
    'thai massage',
    'shiatsu massage',
    'foot reflex',
    'foot massage',
    'pre-natal massage',
    'hilot'
  )
  AND s.deleted_at IS NULL
ORDER BY s.name;
