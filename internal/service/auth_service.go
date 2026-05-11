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

var allowedRoles = []string{"client", "therapist", "admin", "rider"}
var allowedProviders = []string{"email", "phone", "google.com", "apple.com"}

func (a *authService) Signup(ctx context.Context, provider, provider_key, password, role string) (int, string, error) {
	return a.signupWithCreator(ctx, provider, provider_key, password, role, a.user.CreateUserAndIdentity)
}

func (a *authService) SignupWithTherapistProfile(ctx context.Context, provider, provider_key, password, role string) (int, string, error) {
	if role != "therapist" {
		return 0, "", fmt.Errorf("invalid role")
	}
	return a.signupWithCreator(ctx, provider, provider_key, password, role, a.user.CreateUserIdentityAndTherapistProfile)
}

func (a *authService) signupWithCreator(ctx context.Context, provider, provider_key, password, role string, createUser func(context.Context, model.User, model.UserAuthIdentity) error) (int, string, error) {
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
	// Minimum Length
	if len(password) < 8 {
		return 0, "", fmt.Errorf("password must be atleast 8 characters")
	}

	// At least one uppercase letter
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return 0, "", fmt.Errorf("password must have atleast one uppercase character")
	}

	// At least one lowercase letter
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return 0, "", fmt.Errorf("password must have atleast one lowercase character")
	}

	// At least one digit
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return 0, "", fmt.Errorf("password must have a number")
	}

	// At least one special character (adjust as needed)
	if !regexp.MustCompile(`[!@#$%^&*()]`).MatchString(password) {
		return 0, "", fmt.Errorf("password must have a special character")
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
	if role == "client" || role == "rider" {
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

	if !model.CanAccountLogin(user.AccountStatus) {
		return "", fmt.Errorf("%s", model.AccountStatusLoginError(user.AccountStatus))
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
