package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrInvalidGoogleCredential = errors.New("invalid google credential")
	ErrGoogleEmailUnverified   = errors.New("google email is not verified")
	ErrGoogleAccountLinkNeeded = errors.New("an account already exists for this email")
	ErrGoogleIdentityInUse     = errors.New("google account is already connected to another user")
)

type GoogleAuthResult struct {
	Token        string `json:"token"`
	UserID       int    `json:"user_id"`
	IsNewUser    bool   `json:"is_new_user"`
	NeedsProfile bool   `json:"needs_profile"`
}

type GoogleAuthService interface {
	Authenticate(ctx context.Context, credential string) (*GoogleAuthResult, error)
	Link(ctx context.Context, userID int, credential string) error
}

type googleAuthService struct {
	repo      repository.GoogleAuthRepository
	verifier  oauth.GoogleCredentialVerifier
	jwtSecret string
}

func NewGoogleAuthService(repo repository.GoogleAuthRepository, verifier oauth.GoogleCredentialVerifier, jwtSecret string) GoogleAuthService {
	return &googleAuthService{repo: repo, verifier: verifier, jwtSecret: jwtSecret}
}

func (s *googleAuthService) Authenticate(ctx context.Context, credential string) (*GoogleAuthResult, error) {
	identity, err := s.verify(ctx, credential)
	if err != nil {
		return nil, err
	}

	existingIdentity, err := s.repo.FindIdentityByKey(ctx, "google", identity.Subject)
	if err == nil {
		return s.issueForUser(ctx, existingIdentity.UserID, false)
	}
	if !errors.Is(err, repository.ErrGoogleAuthRecordNotFound) {
		return nil, fmt.Errorf("lookup google identity: %w", err)
	}

	if _, err := s.repo.FindUserByEmail(ctx, identity.Email); err == nil {
		return nil, ErrGoogleAccountLinkNeeded
	} else if !errors.Is(err, repository.ErrGoogleAuthRecordNotFound) {
		return nil, fmt.Errorf("lookup google email: %w", err)
	}

	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = "Kalinga customer"
	}
	userID, err := s.repo.CreateGoogleUserAndIdentity(ctx, model.User{
		FullName:     name,
		Role:         model.RoleClient,
		PrimaryEmail: strings.ToLower(strings.TrimSpace(identity.Email)),
		ProfilePhoto: identity.Picture,
	}, identity.Subject)
	if err != nil {
		// A concurrent request may have created the identity first.
		if created, lookupErr := s.repo.FindIdentityByKey(ctx, "google", identity.Subject); lookupErr == nil {
			return s.issueForUser(ctx, created.UserID, false)
		}
		return nil, fmt.Errorf("create google user: %w", err)
	}
	return s.issueForUser(ctx, userID, true)
}

func (s *googleAuthService) Link(ctx context.Context, userID int, credential string) error {
	identity, err := s.verify(ctx, credential)
	if err != nil {
		return err
	}

	existing, err := s.repo.FindIdentityByKey(ctx, "google", identity.Subject)
	if err == nil {
		if existing.UserID == userID {
			return nil
		}
		return ErrGoogleIdentityInUse
	}
	if !errors.Is(err, repository.ErrGoogleAuthRecordNotFound) {
		return fmt.Errorf("lookup google identity: %w", err)
	}

	if _, err := s.repo.FindUserByID(ctx, userID); err != nil {
		return fmt.Errorf("lookup linking user: %w", err)
	}
	if err := s.repo.LinkGoogleIdentity(ctx, userID, identity.Subject); err != nil {
		return err
	}
	return nil
}

func (s *googleAuthService) verify(ctx context.Context, credential string) (*oauth.GoogleIdentity, error) {
	identity, err := s.verifier.Verify(ctx, credential)
	if err != nil {
		if errors.Is(err, oauth.ErrGoogleCredentialNotConfigured) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidGoogleCredential, err)
	}
	if !identity.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}
	return identity, nil
}

func (s *googleAuthService) issueForUser(ctx context.Context, userID int, isNew bool) (*GoogleAuthResult, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load google user: %w", err)
	}
	if user.AccountStatus != "" && user.AccountStatus != "active" {
		return nil, fmt.Errorf("Account is %s", user.AccountStatus)
	}
	token, err := auth.GenerateToken(user.UserID, user.Role, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("generate google auth token: %w", err)
	}
	return &GoogleAuthResult{
		Token:        token,
		UserID:       user.UserID,
		IsNewUser:    isNew,
		NeedsProfile: strings.TrimSpace(user.PrimaryPhone) == "",
	}, nil
}
