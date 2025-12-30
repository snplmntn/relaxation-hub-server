package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// fakeUserRepo implements repository.UserRepository for tests.
type fakeUserRepo struct {
    user *model.User
    err  error
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

func TestUserService_Get_Success(t *testing.T) {
    expected := &model.User{UserID: 42, FullName: "Test User", PrimaryEmail: "t@example.com"}
    repo := &fakeUserRepo{user: expected}
    svc := NewUserService(repo)

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
    svc := NewUserService(repo)

    got, err := svc.Get(context.Background(), 7)
    if err == nil {
        t.Fatalf("expected error, got nil and user %+v", got)
    }
}

func TestUserService_BlockUser_Success(t *testing.T) {
    repo := &fakeUserRepo{}
    svc := NewUserService(repo)

    err := svc.BlockUser(context.Background(), 1, 2)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func TestUserService_BlockUser_SelfBlock(t *testing.T) {
    repo := &fakeUserRepo{}
    svc := NewUserService(repo)

    err := svc.BlockUser(context.Background(), 1, 1)
    if err == nil {
        t.Fatal("expected error blocking self, got nil")
    }
    if err.Error() != "cannot block yourself" {
        t.Fatalf("unexpected error message: %v", err.Error())
    }
}
