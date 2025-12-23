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
