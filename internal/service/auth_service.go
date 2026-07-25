package service

import (
	"context"
	"fmt"
	"net/mail"
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
	// Signup returns the created user ID and, for clients, a JWT token string.
	Signup(ctx context.Context, provider, provider_key, password, role string) (userID int, token string, err error)
	SignupWithTherapistProfile(ctx context.Context, provider, provider_key, password, role string) (userID int, token string, err error)
	SignupStaff(ctx context.Context, provider, provider_key, password, role string) (userID int, token string, err error)
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

// isEmailValid validates email addresses using the net/mail package
// and additionally requires a proper TLD (e.g., .com, .org, .net).
func isEmailValid(e string) bool {
	addr, err := mail.ParseAddress(e)
	if err != nil {
		return false
	}
	// Require at least one dot in the domain part to ensure a TLD exists
	// This rejects "user@domain" but accepts "user@domain.com"
	atIndex := strings.LastIndex(addr.Address, "@")
	if atIndex == -1 {
		return false
	}
	domain := addr.Address[atIndex+1:]
	return strings.Contains(domain, ".")
}

func isPhoneValid(p string) bool {
	// Philippine phone number format: +639xxxxxxxxx
	phoneRegex := regexp.MustCompile(`^\+639\d{9}$`)
	return phoneRegex.MatchString(p)
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be atleast 8 characters")
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return fmt.Errorf("password must have atleast one uppercase character")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return fmt.Errorf("password must have atleast one lowercase character")
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return fmt.Errorf("password must have a number")
	}
	if !regexp.MustCompile(`[!@#$%^&*()]`).MatchString(password) {
		return fmt.Errorf("password must have a special character")
	}
	return nil
}

var allowedSignupRoles = []string{model.RoleClient, model.RoleTherapist, model.RoleRider}
var allowedStaffRoles = []string{model.RoleAdmin, model.RoleSuperAdmin}
var allowedProviders = []string{"email", "phone", "google.com", "apple.com"}

func (a *authService) Signup(ctx context.Context, provider, provider_key, password, role string) (int, string, error) {
	return a.signupWithCreator(ctx, provider, provider_key, password, role, allowedSignupRoles, a.user.CreateUserAndIdentity)
}

func (a *authService) SignupWithTherapistProfile(ctx context.Context, provider, provider_key, password, role string) (int, string, error) {
	if role != model.RoleTherapist {
		return 0, "", fmt.Errorf("invalid role")
	}
	return a.signupWithCreator(ctx, provider, provider_key, password, role, allowedSignupRoles, a.user.CreateUserIdentityAndTherapistProfile)
}

func (a *authService) SignupStaff(ctx context.Context, provider, provider_key, password, role string) (int, string, error) {
	return a.signupWithCreator(ctx, provider, provider_key, password, role, allowedStaffRoles, a.user.CreateUserAndIdentity)
}

func (a *authService) signupWithCreator(ctx context.Context, provider, provider_key, password, role string, allowedRoles []string, createUser func(context.Context, model.User, model.UserAuthIdentity) error) (int, string, error) {
	// Validation
	// 1. All fields complete
	if provider_key == "" || password == "" || role == "" {
		return 0, "", fmt.Errorf("please complete all fields")
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	if !slices.Contains(allowedProviders, provider) {
		return 0, "", fmt.Errorf("unsupported provider")
	}

	// 2. Email Validation
	// 2. Email/Phone Validation
	if provider == "email" {
		provider_key = strings.ToLower(provider_key)
		if !isEmailValid(provider_key) {
			return 0, "", fmt.Errorf("please input a valid email")
		}
		if _, err := a.user.FindIdentityByKey(ctx, "email", provider_key); err == nil {
			return 0, "", fmt.Errorf("email already in use")
		}
	} else if provider == "phone" {
		if !isPhoneValid(provider_key) {
			return 0, "", fmt.Errorf("please input a valid phone number (+639xxxxxxxxx)")
		}
		if _, err := a.user.FindIdentityByKey(ctx, "phone", provider_key); err == nil {
			return 0, "", fmt.Errorf("phone number already in use")
		}
	}

	// 3. Password Validation
	if err := validatePassword(password); err != nil {
		return 0, "", err
	}

	if !slices.Contains(allowedRoles, role) {
		return 0, "", fmt.Errorf("invalid role")
	}

	// Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, "", fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := model.User{
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if provider == "email" {
		user.PrimaryEmail = provider_key
	} else if provider == "phone" {
		user.PrimaryPhone = provider_key
	}

	identity := model.UserAuthIdentity{
		Provider:     provider,
		ProviderKey:  provider_key,
		PasswordHash: string(hashedPassword),
		CreatedAt:    now,
	}

	err = createUser(ctx, user, identity)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Retrieve the created user to get the ID
	createdIdentity, err := a.user.FindIdentityByKey(ctx, provider, provider_key)
	if err != nil {
		return 0, "", fmt.Errorf("failed to retrieve created user: %w", err)
	}

	// If the created user is a client or rider, generate a token for immediate use
	var token string
	if role == model.RoleClient || role == model.RoleRider {
		tokenStr, err := auth.GenerateToken(createdIdentity.UserID, role, a.config.JWTKey)
		if err != nil {
			return createdIdentity.UserID, "", fmt.Errorf("failed to generate token: %w", err)
		}
		token = tokenStr
	}

	return createdIdentity.UserID, token, nil
}

func (a *authService) Login(ctx context.Context, provider, provider_key, password string) (tokenString string, err error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if !slices.Contains(allowedProviders, provider) {
		return "", fmt.Errorf("unsupported provider")
	}

	if provider == "email" {
		provider_key = strings.ToLower(provider_key)
	}

	identity, err := a.user.FindIdentityByKey(ctx, provider, provider_key)
	if err != nil {
		if !isIdentityNotFoundError(err) {
			return "", fmt.Errorf("login identity lookup failed: %w", err)
		}
		return "", fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(identity.PasswordHash), []byte(password))
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	user, err := a.user.FindUserByID(ctx, identity.UserID)
	if err != nil {
		if isUserNotFoundLoginError(err) {
			return "", fmt.Errorf("invalid credentials")
		}
		return "", fmt.Errorf("login user lookup failed: %w", err)
	}

	if user.AccountStatus != "active" {
		switch user.AccountStatus {
		case "banned":
			return "", fmt.Errorf("Account is banned")
		case "suspended":
			return "", fmt.Errorf("Account is suspended")
		case "inactive":
			return "", fmt.Errorf("Account is inactive")
		case "blocked":
			return "", fmt.Errorf("Account is blocked")
		default:
			return "", fmt.Errorf("Account is not active")
		}
	}

	token, err := auth.GenerateToken(user.UserID, user.Role, a.config.JWTKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func isIdentityNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "identity not found")
}

func isUserNotFoundLoginError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "user not found")
}

func (a *authService) ParseToken(ctx context.Context, tokenString string) (claims jwt.Claims, err error) {
	claim, err := auth.ValidateToken(tokenString, a.config.JWTKey)
	if err != nil {
		return nil, err
	}

	return claim, nil
}
