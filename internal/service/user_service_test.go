package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// fakeUserRepo implements repository.UserRepository for tests.
type fakeUserRepo struct {
    user *model.User
    err  error
}

type fakeAddressRepo struct {
	repository.AddressRepository
}

func (f *fakeAddressRepo) ListForUser(ctx context.Context, userID int64, excludeDefault bool) ([]model.Address, error) {
	return nil, nil
}

func (f *fakeUserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
    return nil
}

func (f *fakeUserRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
    return nil, errors.New("not implemented")
}

func (f *fakeUserRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
    if f.err != nil {
        return nil, f.err
    }
    return f.user, nil
}

func (f *fakeUserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
    return nil
}

func (f *fakeUserRepo) ListUsers(ctx context.Context, role string) ([]model.User, error) {
    if f.err != nil {
        return nil, f.err
    }
    if f.user == nil {
        return []model.User{}, nil
    }
    return []model.User{*f.user}, nil
}

func (f *fakeUserRepo) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
    return f.err
}

func (f *fakeUserRepo) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
    return f.err
}

func (f *fakeUserRepo) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
    return false, f.err
}
func (f *fakeUserRepo) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*repository.UserInfo, error) {
	return map[int64]*repository.UserInfo{}, nil
}
func (f *fakeUserRepo) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*repository.TherapistInfo, error) {
	return map[int64]*repository.TherapistInfo{}, nil
}
func (f *fakeUserRepo) GetBlockList(ctx context.Context, blockerID int64) ([]repository.BlockedUserEntry, error) {
	return []repository.BlockedUserEntry{}, nil
}
func (f *fakeUserRepo) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	return nil
}
func (f *fakeUserRepo) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	return nil, nil
}

func (f *fakeUserRepo) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (f *fakeUserRepo) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (f *fakeUserRepo) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	return []model.User{}, nil
}

func (f *fakeUserRepo) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return false, nil
}

func (f *fakeUserRepo) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (f *fakeUserRepo) SuspendUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (f *fakeUserRepo) ListUsersPaginated(ctx context.Context, roleFilter string, limit, offset int) ([]model.User, int, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) Create(ctx context.Context, user *model.User) error { return nil }
func (f *fakeUserRepo) GetByID(ctx context.Context, userID int64) (*model.User, error) { return nil, nil }
func (f *fakeUserRepo) GetByPhone(ctx context.Context, phone string) (*model.User, error) { return nil, nil }
func (f *fakeUserRepo) Update(ctx context.Context, user *model.User) error { return nil }
func (f *fakeUserRepo) SetOneSignalPlayerID(ctx context.Context, userID int64, playerID string) error { return nil }
func (f *fakeUserRepo) Delete(ctx context.Context, userID int64) error { return nil }

func TestUserService_Get_Success(t *testing.T) {
    expected := &model.User{UserID: 42, FullName: "Test User", PrimaryEmail: "t@example.com"}
    repo := &fakeUserRepo{user: expected}
    addrRepo := &fakeAddressRepo{}
    svc := NewUserService(repo, addrRepo)

    got, err := svc.Get(context.Background(), 42)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got == nil {
        t.Fatalf("expected user, got nil")
    }
    if got.UserID != expected.UserID || got.FullName != expected.FullName {
        t.Fatalf("mismatch user: got %+v want %+v", got, expected)
    }
}

func TestUserService_Get_NotFound(t *testing.T) {
    repo := &fakeUserRepo{err: errors.New("user not found")}
    addrRepo := &fakeAddressRepo{}
    svc := NewUserService(repo, addrRepo)

    got, err := svc.Get(context.Background(), 7)
    if err == nil {
        t.Fatalf("expected error, got nil and user %+v", got)
    }
}

func TestUserService_BlockUser_Success(t *testing.T) {
    repo := &fakeUserRepo{}
    addrRepo := &fakeAddressRepo{}
    svc := NewUserService(repo, addrRepo)

    err := svc.BlockUser(context.Background(), 1, 2)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func TestUserService_BlockUser_SelfBlock(t *testing.T) {
    repo := &fakeUserRepo{}
    addrRepo := &fakeAddressRepo{}
    svc := NewUserService(repo, addrRepo)

    err := svc.BlockUser(context.Background(), 1, 1)
    if err == nil {
        t.Fatal("expected error blocking self, got nil")
    }
    if err.Error() != "cannot block yourself" {
        t.Fatalf("unexpected error message: %v", err.Error())
    }
}
