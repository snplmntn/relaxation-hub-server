package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type ModerationService interface {
	BlockUser(ctx context.Context, adminID, blockedUserID int64, reason string) error
	UnblockUser(ctx context.Context, blockedUserID int64) error
	ListBlockedUsers(ctx context.Context) ([]repository.ModerationBlockEntry, error)
}

type moderationService struct {
	repo repository.ModerationRepository
}

func NewModerationService(repo repository.ModerationRepository) ModerationService {
	return &moderationService{repo: repo}
}

func (s *moderationService) BlockUser(ctx context.Context, adminID, blockedUserID int64, reason string) error {
	if adminID <= 0 {
		return fmt.Errorf("invalid admin_id")
	}
	if blockedUserID <= 0 {
		return fmt.Errorf("invalid blocked user id")
	}
	if adminID == blockedUserID {
		return fmt.Errorf("cannot block yourself")
	}
	return s.repo.UpsertBlock(ctx, blockedUserID, adminID, strings.TrimSpace(reason))
}

func (s *moderationService) UnblockUser(ctx context.Context, blockedUserID int64) error {
	if blockedUserID <= 0 {
		return fmt.Errorf("invalid blocked user id")
	}
	return s.repo.RemoveBlock(ctx, blockedUserID)
}

func (s *moderationService) ListBlockedUsers(ctx context.Context) ([]repository.ModerationBlockEntry, error) {
	return s.repo.ListActiveBlocks(ctx)
}
