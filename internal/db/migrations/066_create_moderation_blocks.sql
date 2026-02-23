-- Migration 066: Global moderation block list for admin/super-admin tools

CREATE TABLE IF NOT EXISTS moderation_blocks (
    block_id BIGSERIAL PRIMARY KEY,
    blocked_user_id BIGINT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    blocked_by_admin_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_moderation_blocks_active_updated_at
    ON moderation_blocks (updated_at DESC)
    WHERE removed_at IS NULL;
