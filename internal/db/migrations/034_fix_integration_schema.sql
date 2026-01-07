-- ============================================================================
-- MIGRATION 034: Fix Schema Mismatches for Integration Tests
-- ============================================================================
-- This migration adds missing columns to reconcile the DB with the Repository code.

-- 1. FIX BRANCHES TABLE
-- Add missing columns aligned with 001.sql and repository expectations
ALTER TABLE branches ADD COLUMN IF NOT EXISTS branch_name VARCHAR(150);
-- Sync existing data: if 'name' exists but 'branch_name' is null, copy 'name' to 'branch_name'
UPDATE branches SET branch_name = name WHERE branch_name IS NULL AND name IS NOT NULL;
ALTER TABLE branches ALTER COLUMN branch_name SET NOT NULL;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS address_line VARCHAR(255);
UPDATE branches SET address_line = address WHERE address_line IS NULL AND address IS NOT NULL;

ALTER TABLE branches ADD COLUMN IF NOT EXISTS barangay VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS province VARCHAR(100);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS latitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6);
ALTER TABLE branches ADD COLUMN IF NOT EXISTS contact_no VARCHAR(20);
UPDATE branches SET contact_no = phone WHERE contact_no IS NULL AND phone IS NOT NULL;

-- 2. FIX ADMIN_ACTIONS TABLE
-- Add missing columns expected by AdminActionRepository
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_type VARCHAR(50);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS target_id INT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS old_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS new_value TEXT;
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
ALTER TABLE admin_actions ADD COLUMN IF NOT EXISTS performed_at TIMESTAMP;

-- Sync performed_at with created_at
UPDATE admin_actions SET performed_at = created_at WHERE performed_at IS NULL;

-- 3. FIX REFERRALS TABLE
-- Add missing columns expected by ReferralRepository
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referred_id INT;
-- referee_id in DB is likely the same as referred_id in Repository
UPDATE referrals SET referred_id = referee_id WHERE referred_id IS NULL AND referee_id IS NOT NULL;

ALTER TABLE referrals ADD COLUMN IF NOT EXISTS referral_code VARCHAR(50);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS status VARCHAR(20);
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS reward_earned BOOLEAN DEFAULT FALSE;
ALTER TABLE referrals ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;

-- 4. CREATE REFERRAL_REWARDS TABLE
-- This table was missing completely from the DB check
CREATE TABLE IF NOT EXISTS referral_rewards (
    reward_id SERIAL PRIMARY KEY,
    referral_id INT REFERENCES referrals(referral_id),
    user_id INT REFERENCES users(user_id),
    reward_type VARCHAR(50),
    reward_amount NUMERIC(10,2),
    status VARCHAR(20) DEFAULT 'pending',
    expires_at TIMESTAMP,
    redeemed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
