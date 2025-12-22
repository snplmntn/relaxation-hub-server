package service

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Signup(ctx context.Context, provider, provider_key, password, role string) (userID int, err error)
	Login(ctx context.Context, provider, provider_key, password string) (tokenString string, err error)
	ParseToken(ctx context.Context, tokenString string) (claims jwt.Claims, err error)
}

type authService struct {
	user   repository.UserRepository
	config config.Config
}

func NewAuthService(userRepo repository.UserRepository, config *config.Config) AuthService {
	return &authService{user: userRepo, config: *config}
}

func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return emailRegex.MatchString(e)
}

var allowedRoles = []string{"client", "therapist", "admin"}
var allowedProviders = []string{"email", "phone", "google.com", "apple.com"}

func (a *authService) Signup(ctx context.Context, provider, provider_key, password, role string) (int, error) {
	// Validation
	// 1. All fields complete
	if provider_key == "" || password == "" || role == "" {
		return 0, fmt.Errorf("please complete all fields")
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	if !slices.Contains(allowedProviders, provider) {
		return 0, fmt.Errorf("unsupported provider")
	}

	// 2. Email Validation
	if provider == "email" && !isEmailValid(provider_key) {
		return 0, fmt.Errorf("please input a valid email")
	}

	if provider == "email" {
		if _, err := a.user.FindIdentityByKey(ctx, "email", provider_key); err == nil {
			return 0, fmt.Errorf("email already in use")
		}
	}

	// 3. Password Validation
	// Minimum Length
	if len(password) < 8 {
		return 0, fmt.Errorf("password must be atleast 8 characters")
	}

	// At least one uppercase letter
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return 0, fmt.Errorf("password must have atleast one uppercase character")
	}

	// At least one lowercase letter
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return 0, fmt.Errorf("password must have atleast one lowercase character")
	}

	// At least one digit
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return 0, fmt.Errorf("password must have a number")
	}

	// At least one special character (adjust as needed)
	if !regexp.MustCompile(`[!@#$%^&*()]`).MatchString(password) {
		return 0, fmt.Errorf("password must have a special character")
	}

	if !slices.Contains(allowedRoles, role) {
		return 0, fmt.Errorf("invalid role")
	}

	// Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := model.User{
		PrimaryEmail: provider_key,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	identity := model.UserAuthIdentity{
		Provider:     provider,
		ProviderKey:  provider_key,
		PasswordHash: string(hashedPassword),
		CreatedAt:    now,
	}

	err = a.user.CreateUserAndIdentity(ctx, user, identity)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	// Retrieve the created user to get the ID
	createdIdentity, err := a.user.FindIdentityByKey(ctx, provider, provider_key)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve created user: %w", err)
	}

	return createdIdentity.UserID, nil
}

func (a *authService) Login(ctx context.Context, provider, provider_key, password string) (tokenString string, err error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if !slices.Contains(allowedProviders, provider) {
		return "", fmt.Errorf("unsupported provider")
	}

	identity, err := a.user.FindIdentityByKey(ctx, provider, provider_key)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(identity.PasswordHash), []byte(password))
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	user, err := a.user.FindUserByID(ctx, identity.UserID)
	if err != nil {
		return "", err
	}

	token, err := auth.GenerateToken(user.UserID, user.Role, a.config.JWTKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (a *authService) ParseToken(ctx context.Context, tokenString string) (claims jwt.Claims, err error) {
	claim, err := auth.ValidateToken(tokenString, a.config.JWTKey)
	if err != nil {
		return nil, err
	}

	return claim, nil
}
