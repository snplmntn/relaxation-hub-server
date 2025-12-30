package service

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// BlockService handles user blocking business logic
type BlockService struct {
	repo repository.BlockRepository
}

// NewBlockService creates a new block service
func NewBlockService(repo repository.BlockRepository) *BlockService {
	return &BlockService{repo: repo}
}

// BlockUser creates a block from blocker to blocked
func (s *BlockService) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	return s.repo.Create(ctx, blockerID, blockedID)
}

// UnblockUser removes a block from blocker to blocked
func (s *BlockService) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	return s.repo.Delete(ctx, blockerID, blockedID)
}

// IsBlocked checks if either user has blocked the other (returns true if blocked in either direction)
func (s *BlockService) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	return s.repo.IsBlocked(ctx, userA, userB)
}

// GetBlockList returns users blocked by the given user
func (s *BlockService) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockEntry, error) {
	return s.repo.ListBlockedByUser(ctx, userID)
}
