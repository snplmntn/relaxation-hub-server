package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type fakeGoogleVerifier struct {
	identity *oauth.GoogleIdentity
	err      error
}

func (f *fakeGoogleVerifier) Verify(context.Context, string) (*oauth.GoogleIdentity, error) {
	return f.identity, f.err
}

type fakeGoogleAuthRepo struct {
	identity       *model.UserAuthIdentity
	identityErr    error
	userByID       *model.User
	userByIDErr    error
	userByEmail    *model.User
	userByEmailErr error
	createdUser    model.User
	createdKey     string
	linkedUserID   int
	linkedKey      string
}

func (f *fakeGoogleAuthRepo) FindIdentityByKey(context.Context, string, string) (*model.UserAuthIdentity, error) {
	return f.identity, f.identityErr
}

func (f *fakeGoogleAuthRepo) FindUserByID(context.Context, int) (*model.User, error) {
	return f.userByID, f.userByIDErr
}

func (f *fakeGoogleAuthRepo) FindUserByEmail(context.Context, string) (*model.User, error) {
	return f.userByEmail, f.userByEmailErr
}

func (f *fakeGoogleAuthRepo) CreateGoogleUserAndIdentity(_ context.Context, user model.User, providerKey string) (int, error) {
	f.createdUser = user
	f.createdKey = providerKey
	return 42, nil
}

func (f *fakeGoogleAuthRepo) LinkGoogleIdentity(_ context.Context, userID int, providerKey string) error {
	f.linkedUserID = userID
	f.linkedKey = providerKey
	return nil
}

func verifiedGoogleIdentity() *oauth.GoogleIdentity {
	return &oauth.GoogleIdentity{
		Subject:       "google-subject",
		Email:         "Customer@Example.com",
		EmailVerified: true,
		Name:          "Google Customer",
		Picture:       "https://example.com/photo.jpg",
	}
}

func TestGoogleAuthenticateExistingIdentity(t *testing.T) {
	repo := &fakeGoogleAuthRepo{
		identity: &model.UserAuthIdentity{UserID: 7},
		userByID: &model.User{UserID: 7, Role: model.RoleClient, AccountStatus: "active", PrimaryPhone: "+639123456789"},
	}
	service := NewGoogleAuthService(repo, &fakeGoogleVerifier{identity: verifiedGoogleIdentity()}, "test-secret")

	result, err := service.Authenticate(context.Background(), "credential")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if result.Token == "" || result.UserID != 7 || result.IsNewUser || result.NeedsProfile {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGoogleAuthenticateCreatesClient(t *testing.T) {
	repo := &fakeGoogleAuthRepo{
		identityErr:    repository.ErrGoogleAuthRecordNotFound,
		userByEmailErr: repository.ErrGoogleAuthRecordNotFound,
		userByID:       &model.User{UserID: 42, Role: model.RoleClient, AccountStatus: "active"},
	}
	service := NewGoogleAuthService(repo, &fakeGoogleVerifier{identity: verifiedGoogleIdentity()}, "test-secret")

	result, err := service.Authenticate(context.Background(), "credential")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if !result.IsNewUser || !result.NeedsProfile || result.UserID != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if repo.createdUser.Role != model.RoleClient || repo.createdUser.PrimaryEmail != "customer@example.com" {
		t.Fatalf("unexpected created user: %+v", repo.createdUser)
	}
	if repo.createdKey != "google-subject" {
		t.Fatalf("unexpected provider key: %s", repo.createdKey)
	}
}

func TestGoogleAuthenticateRequiresLinkForMatchingEmail(t *testing.T) {
	repo := &fakeGoogleAuthRepo{
		identityErr: repository.ErrGoogleAuthRecordNotFound,
		userByEmail: &model.User{UserID: 9},
	}
	service := NewGoogleAuthService(repo, &fakeGoogleVerifier{identity: verifiedGoogleIdentity()}, "test-secret")

	_, err := service.Authenticate(context.Background(), "credential")
	if !errors.Is(err, ErrGoogleAccountLinkNeeded) {
		t.Fatalf("expected ErrGoogleAccountLinkNeeded, got %v", err)
	}
}

func TestGoogleAuthenticateRejectsUnverifiedEmail(t *testing.T) {
	identity := verifiedGoogleIdentity()
	identity.EmailVerified = false
	service := NewGoogleAuthService(&fakeGoogleAuthRepo{}, &fakeGoogleVerifier{identity: identity}, "test-secret")

	_, err := service.Authenticate(context.Background(), "credential")
	if !errors.Is(err, ErrGoogleEmailUnverified) {
		t.Fatalf("expected ErrGoogleEmailUnverified, got %v", err)
	}
}

func TestGoogleLinkIsIdempotentForSameUser(t *testing.T) {
	repo := &fakeGoogleAuthRepo{identity: &model.UserAuthIdentity{UserID: 7}}
	service := NewGoogleAuthService(repo, &fakeGoogleVerifier{identity: verifiedGoogleIdentity()}, "test-secret")

	if err := service.Link(context.Background(), 7, "credential"); err != nil {
		t.Fatalf("Link returned error: %v", err)
	}
	if repo.linkedUserID != 0 {
		t.Fatal("idempotent link should not insert another identity")
	}
}

func TestGoogleLinkRejectsIdentityOwnedByAnotherUser(t *testing.T) {
	repo := &fakeGoogleAuthRepo{identity: &model.UserAuthIdentity{UserID: 8}}
	service := NewGoogleAuthService(repo, &fakeGoogleVerifier{identity: verifiedGoogleIdentity()}, "test-secret")

	err := service.Link(context.Background(), 7, "credential")
	if !errors.Is(err, ErrGoogleIdentityInUse) {
		t.Fatalf("expected ErrGoogleIdentityInUse, got %v", err)
	}
}
