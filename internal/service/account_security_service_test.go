package service

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type accountSecurityRepositoryStub struct {
	passwordHash    string
	updatedHash     string
	staffUpdateHash string
	staffUpdateErr  error
	deleted         bool
}

func (s *accountSecurityRepositoryStub) GetEmailPasswordHash(context.Context, int64) (string, error) {
	return s.passwordHash, nil
}

func (s *accountSecurityRepositoryStub) UpdateEmailPasswordHash(_ context.Context, _ int64, passwordHash string) error {
	s.updatedHash = passwordHash
	return nil
}

func (s *accountSecurityRepositoryStub) UpdateStaffEmailPasswordHash(_ context.Context, _ int64, passwordHash string) error {
	s.staffUpdateHash = passwordHash
	return s.staffUpdateErr
}

func (s *accountSecurityRepositoryStub) DeleteClientAccount(context.Context, int64) error {
	s.deleted = true
	return nil
}

func passwordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword returned error: %v", err)
	}
	return string(hash)
}

func TestAccountSecurityChangePasswordVerifiesCurrentPassword(t *testing.T) {
	repo := &accountSecurityRepositoryStub{passwordHash: passwordHash(t, "Current123!")}
	service := NewAccountSecurityService(repo)

	err := service.ChangePassword(context.Background(), 7, "wrong", "Updated123!")
	validationErr, ok := err.(*ValidationError)
	if !ok || validationErr.Code != "invalid_current_password" {
		t.Fatalf("expected invalid_current_password, got %v", err)
	}
	if repo.updatedHash != "" {
		t.Fatal("password hash changed after incorrect current password")
	}
}

func TestAccountSecurityChangePasswordStoresNewHash(t *testing.T) {
	repo := &accountSecurityRepositoryStub{passwordHash: passwordHash(t, "Current123!")}
	service := NewAccountSecurityService(repo)

	if err := service.ChangePassword(context.Background(), 7, "Current123!", "Updated123!"); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.updatedHash), []byte("Updated123!")) != nil {
		t.Fatal("updated hash does not match the new password")
	}
}

func TestAccountSecurityResetStaffPasswordStoresNewHash(t *testing.T) {
	repo := &accountSecurityRepositoryStub{}
	service := NewAccountSecurityService(repo)

	if err := service.ResetStaffPassword(context.Background(), 11, "Updated123!"); err != nil {
		t.Fatalf("ResetStaffPassword returned error: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.staffUpdateHash), []byte("Updated123!")) != nil {
		t.Fatal("updated staff hash does not match the new password")
	}
}

func TestAccountSecurityResetStaffPasswordRejectsWeakPassword(t *testing.T) {
	repo := &accountSecurityRepositoryStub{}
	service := NewAccountSecurityService(repo)

	err := service.ResetStaffPassword(context.Background(), 11, "too-short")
	validationErr, ok := err.(*ValidationError)
	if !ok || validationErr.Code != "weak_password" {
		t.Fatalf("expected weak_password, got %v", err)
	}
	if repo.staffUpdateHash != "" {
		t.Fatal("staff password hash changed after validation failed")
	}
}

func TestAccountSecurityDeleteAccountRequiresCurrentPassword(t *testing.T) {
	repo := &accountSecurityRepositoryStub{passwordHash: passwordHash(t, "Current123!")}
	service := NewAccountSecurityService(repo)

	if err := service.DeleteAccount(context.Background(), 7, "Current123!"); err != nil {
		t.Fatalf("DeleteAccount returned error: %v", err)
	}
	if !repo.deleted {
		t.Fatal("expected account deletion after password verification")
	}
}
