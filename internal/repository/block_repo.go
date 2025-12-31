package repository

import (
	"context"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// BlockEntry represents a block relationship
type BlockEntry struct {
	BlockerUserID int64     `json:"blocker_user_id"`
	BlockedUserID int64     `json:"blocked_user_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// BlockRepository handles user block operations
type BlockRepository interface {
	Create(ctx context.Context, blockerID, blockedID int64) error
	Delete(ctx context.Context, blockerID, blockedID int64) error
	IsBlocked(ctx context.Context, userA, userB int64) (bool, error)
	ListBlockedByUser(ctx context.Context, blockerID int64) ([]BlockEntry, error)
}

type blockRepository struct {
	db db.DBTX
}

// NewBlockRepository creates a new block repository
func NewBlockRepository(db db.DBTX) BlockRepository {
	return &blockRepository{db: db}
}

// Create adds a block record
func (r *blockRepository) Create(ctx context.Context, blockerID, blockedID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_blocks (blocker_user_id, blocked_user_id)
		VALUES ($1, $2)
		ON CONFLICT (blocker_user_id, blocked_user_id) DO NOTHING
	`, blockerID, blockedID)
	return err
}

// Delete removes a block record (unblock)
func (r *blockRepository) Delete(ctx context.Context, blockerID, blockedID int64) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM user_blocks
		WHERE blocker_user_id = $1 AND blocked_user_id = $2
	`, blockerID, blockedID)
	return err
}

// IsBlocked checks if either user has blocked the other (bidirectional check)
func (r *blockRepository) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_blocks
			WHERE (blocker_user_id = $1 AND blocked_user_id = $2)
			   OR (blocker_user_id = $2 AND blocked_user_id = $1)
		)
	`, userA, userB).Scan(&exists)
	return exists, err
}

// ListBlockedByUser returns all users blocked by a specific user
func (r *blockRepository) ListBlockedByUser(ctx context.Context, blockerID int64) ([]BlockEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT blocker_user_id, blocked_user_id, created_at
		FROM user_blocks
		WHERE blocker_user_id = $1
		ORDER BY created_at DESC
	`, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlockEntry
	for rows.Next() {
		var e BlockEntry
		if err := rows.Scan(&e.BlockerUserID, &e.BlockedUserID, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
