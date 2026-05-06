package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

type ModerationBlockEntry struct {
	UserID       int64  `json:"user_id"`
	FullName     string `json:"full_name"`
	ProfilePhoto string `json:"profile_photo"`
	BlockedAt    string `json:"blocked_at"`
	Reason       string `json:"reason,omitempty"`
}

type ModerationRepository interface {
	UpsertBlock(ctx context.Context, blockedUserID, adminID int64, reason string) error
	RemoveBlock(ctx context.Context, blockedUserID int64) error
	ListActiveBlocks(ctx context.Context) ([]ModerationBlockEntry, error)
}

type moderationRepo struct {
	db db.DBTX
}

func NewModerationRepository(db db.DBTX) ModerationRepository {
	return &moderationRepo{db: db}
}

func (r *moderationRepo) UpsertBlock(ctx context.Context, blockedUserID, adminID int64, reason string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO moderation_blocks (blocked_user_id, blocked_by_admin_id, reason, created_at, updated_at, removed_at)
		VALUES ($1, $2, NULLIF($3, ''), NOW(), NOW(), NULL)
		ON CONFLICT (blocked_user_id) DO UPDATE
		SET blocked_by_admin_id = EXCLUDED.blocked_by_admin_id,
		    reason = EXCLUDED.reason,
		    updated_at = NOW(),
		    removed_at = NULL
	`
	_, err := r.db.Exec(ctx, query, blockedUserID, adminID, reason)
	if err != nil {
		return fmt.Errorf("failed to upsert moderation block: %w", err)
	}
	return nil
}

func (r *moderationRepo) RemoveBlock(ctx context.Context, blockedUserID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		UPDATE moderation_blocks
		SET removed_at = NOW(), updated_at = NOW()
		WHERE blocked_user_id = $1 AND removed_at IS NULL
	`
	_, err := r.db.Exec(ctx, query, blockedUserID)
	if err != nil {
		return fmt.Errorf("failed to remove moderation block: %w", err)
	}
	return nil
}

func (r *moderationRepo) ListActiveBlocks(ctx context.Context) ([]ModerationBlockEntry, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT
			mb.blocked_user_id,
			COALESCE(u.full_name, 'Unknown'),
			COALESCE(u.profile_photo, ''),
			mb.updated_at,
			COALESCE(mb.reason, '')
		FROM moderation_blocks mb
		LEFT JOIN users u ON u.user_id = mb.blocked_user_id
		WHERE mb.removed_at IS NULL
		ORDER BY mb.updated_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list moderation blocks: %w", err)
	}
	defer rows.Close()

	entries := make([]ModerationBlockEntry, 0)
	for rows.Next() {
		var entry ModerationBlockEntry
		var blockedAt time.Time
		if err := rows.Scan(&entry.UserID, &entry.FullName, &entry.ProfilePhoto, &blockedAt, &entry.Reason); err != nil {
			return nil, fmt.Errorf("failed to scan moderation block: %w", err)
		}
		entry.BlockedAt = blockedAt.Format(time.RFC3339)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating moderation blocks: %w", err)
	}

	return entries, nil
}
