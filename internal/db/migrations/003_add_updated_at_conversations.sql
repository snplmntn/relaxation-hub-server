-- 003_add_updated_at_conversations.sql
-- Idempotent migration: add `updated_at` to `conversations` and a trigger

BEGIN;

-- Add updated_at column if missing
ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Trigger function to update updated_at on row changes
CREATE OR REPLACE FUNCTION rh_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger only if it doesn't already exist
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'trg_conversations_updated_at'
  ) THEN
    EXECUTE 'CREATE TRIGGER trg_conversations_updated_at
      BEFORE UPDATE ON conversations
      FOR EACH ROW
      EXECUTE FUNCTION rh_set_updated_at()';
  END IF;
END;
$$;

COMMIT;

-- Notes:
-- Run this migration in your DB environment (psql or migration tool).
-- It's safe to re-run because of IF NOT EXISTS / CREATE OR REPLACE.
