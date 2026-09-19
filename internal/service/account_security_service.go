package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AccountSecurityService struct {
	repo repository.AccountSecurityRepository
}

func NewAccountSecurityService(repo repository.AccountSecurityRepository) *AccountSecurityService {
	return &AccountSecurityService{repo: repo}
}

func (s *AccountSecurityService) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	passwordHash, err := s.verifiedPasswordHash(ctx, userID, currentPassword)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(newPassword)) == nil {
		return NewValidationError("password_unchanged", "Choose a password you have not used for this account.", nil)
	}
	if err := validatePassword(newPassword); err != nil {
		return NewValidationError("weak_password", err.Error(), nil)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := s.repo.UpdateEmailPasswordHash(ctx, userID, string(newHash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *AccountSecurityService) ResetStaffPassword(ctx context.Context, userID int64, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return NewValidationError("weak_password", err.Error(), nil)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := s.repo.UpdateStaffEmailPasswordHash(ctx, userID, string(newHash)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewValidationError("password_login_unavailable", "Password management is not available for this staff account.", nil)
		}
		return fmt.Errorf("update staff password: %w", err)
	}
	return nil
}

func (s *AccountSecurityService) DeleteAccount(ctx context.Context, userID int64, currentPassword string) error {
	if _, err := s.verifiedPasswordHash(ctx, userID, currentPassword); err != nil {
		return err
	}
	if err := s.repo.DeleteClientAccount(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NewValidationError("account_not_found", "This client account is no longer available.", nil)
		}
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

func (s *AccountSecurityService) verifiedPasswordHash(ctx context.Context, userID int64, currentPassword string) (string, error) {
	if currentPassword == "" {
		return "", NewValidationError("current_password_required", "Enter your current password.", nil)
	}
	passwordHash, err := s.repo.GetEmailPasswordHash(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", NewValidationError("password_login_unavailable", "Password management is not available for this account.", nil)
		}
		return "", fmt.Errorf("load password identity: %w", err)
	}
	if passwordHash == "" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(currentPassword)) != nil {
		return "", NewValidationError("invalid_current_password", "Current password is incorrect.", nil)
	}
	return passwordHash, nil
}
