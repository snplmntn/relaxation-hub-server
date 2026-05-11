package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type moderationRepoFake struct {
	repository.ModerationRepository
	blockedUserID   int64
	unblockedUserID int64
}

func (f *moderationRepoFake) UpsertBlock(ctx context.Context, blockedUserID, adminID int64, reason string) error {
	f.blockedUserID = blockedUserID
	return nil
}

func (f *moderationRepoFake) RemoveBlock(ctx context.Context, blockedUserID int64) error {
	f.unblockedUserID = blockedUserID
	return nil
}

func (f *moderationRepoFake) ListActiveBlocks(ctx context.Context) ([]repository.ModerationBlockEntry, error) {
	return nil, nil
}

type moderationUserRepoFake struct {
	repository.UserRepository
	user    *model.User
	updates []map[string]interface{}
}

func (f *moderationUserRepoFake) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	f.updates = append(f.updates, updates)
	return nil
}

func (f *moderationUserRepoFake) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return f.user, nil
}

func TestModerationServiceBlockUserSetsAccountStatusBlocked(t *testing.T) {
	moderationRepo := &moderationRepoFake{}
	userRepo := &moderationUserRepoFake{}
	svc := NewModerationService(moderationRepo, userRepo)

	if err := svc.BlockUser(context.Background(), 1, 2, "safety"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if moderationRepo.blockedUserID != 2 {
		t.Fatalf("expected blocked user 2, got %d", moderationRepo.blockedUserID)
	}
	if len(userRepo.updates) != 1 {
		t.Fatalf("expected 1 user update, got %d", len(userRepo.updates))
	}
	if got := userRepo.updates[0]["account_status"]; got != model.AccountStatusBlocked {
		t.Fatalf("expected blocked account_status, got %v", got)
	}
}

func TestModerationServiceUnblockUserRestoresBlockedUserToActive(t *testing.T) {
	moderationRepo := &moderationRepoFake{}
	userRepo := &moderationUserRepoFake{
		user: &model.User{
			UserID:        2,
			AccountStatus: model.AccountStatusBlocked,
		},
	}
	svc := NewModerationService(moderationRepo, userRepo)

	if err := svc.UnblockUser(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if moderationRepo.unblockedUserID != 2 {
		t.Fatalf("expected unblocked user 2, got %d", moderationRepo.unblockedUserID)
	}
	if len(userRepo.updates) != 1 {
		t.Fatalf("expected 1 user update, got %d", len(userRepo.updates))
	}
	if got := userRepo.updates[0]["account_status"]; got != model.AccountStatusActive {
		t.Fatalf("expected active account_status, got %v", got)
	}
}
