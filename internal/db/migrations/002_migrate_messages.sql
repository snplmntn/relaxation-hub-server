-- 002_migrate_messages.sql
-- Safe, idempotent migration to align the existing `messages` table
-- with the updated application schema (content, sent_at, read_at, deleted_at).
-- This script performs conditional changes so it can be applied against
-- development databases without failing if some columns already exist.

BEGIN;

-- Rename legacy text/timestamp columns if present and add missing columns
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'message_text'
  ) THEN
    EXECUTE 'ALTER TABLE messages RENAME COLUMN message_text TO content';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'created_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages RENAME COLUMN created_at TO sent_at';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'read_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages ADD COLUMN read_at TIMESTAMP';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'deleted_at'
  ) THEN
    EXECUTE 'ALTER TABLE messages ADD COLUMN deleted_at TIMESTAMP';
  END IF;

  -- Migrate boolean is_read -> timestamp read_at when applicable
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'messages' AND column_name = 'is_read'
  ) THEN
    -- mark read_at to sent_at for rows that were marked read
    EXECUTE 'UPDATE messages SET read_at = sent_at WHERE is_read = TRUE';
    EXECUTE 'ALTER TABLE messages DROP COLUMN is_read';
  END IF;

END;
$$;

-- Recreate / normalize indexes used by application
DROP INDEX IF EXISTS idx_messages_conversation;
DROP INDEX IF EXISTS idx_messages_unread;
DROP INDEX IF EXISTS idx_messages_sender;

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(conversation_id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);

COMMIT;

-- Notes:
-- 1) If your DB is in production, review and test these changes in a staging
--    environment first. Back up the `messages` table before running.
-- 2) After applying, restart the server so repository queries/scan map to
--    the new columns (`content`, `sent_at`, `read_at`, `deleted_at`).
