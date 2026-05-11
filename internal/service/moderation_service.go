package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type ModerationService interface {
	BlockUser(ctx context.Context, adminID, blockedUserID int64, reason string) error
	UnblockUser(ctx context.Context, blockedUserID int64) error
	ListBlockedUsers(ctx context.Context) ([]repository.ModerationBlockEntry, error)
}

type moderationService struct {
	repo  repository.ModerationRepository
	users repository.UserRepository
}

func NewModerationService(repo repository.ModerationRepository, userRepo ...repository.UserRepository) ModerationService {
	var users repository.UserRepository
	if len(userRepo) > 0 {
		users = userRepo[0]
	}
	return &moderationService{repo: repo, users: users}
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
	if err := s.repo.UpsertBlock(ctx, blockedUserID, adminID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	if s.users != nil {
		updates := map[string]interface{}{
			"account_status": model.AccountStatusBlocked,
		}
		if strings.TrimSpace(reason) != "" {
			updates["status_reason"] = strings.TrimSpace(reason)
		}
		if err := s.users.UpdateUser(ctx, blockedUserID, updates); err != nil {
			return err
		}
	}
	return nil
}

func (s *moderationService) UnblockUser(ctx context.Context, blockedUserID int64) error {
	if blockedUserID <= 0 {
		return fmt.Errorf("invalid blocked user id")
	}
	if err := s.repo.RemoveBlock(ctx, blockedUserID); err != nil {
		return err
	}
	if s.users != nil {
		user, err := s.users.FindUserByID(ctx, int(blockedUserID))
		if err != nil {
			return err
		}
		if strings.EqualFold(user.AccountStatus, model.AccountStatusBlocked) {
			if err := s.users.UpdateUser(ctx, blockedUserID, map[string]interface{}{
				"account_status": model.AccountStatusActive,
				"status_reason":  "",
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *moderationService) ListBlockedUsers(ctx context.Context) ([]repository.ModerationBlockEntry, error) {
	return s.repo.ListActiveBlocks(ctx)
}
