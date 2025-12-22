package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// Mock UserRepository
type mockUserRepo struct {
	createUserAndIdentityFunc func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	findIdentityByKeyFunc     func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	findUserByIDFunc          func(ctx context.Context, userID int) (*model.User, error)
	updateUserFunc            func(ctx context.Context, userID int64, updates map[string]interface{}) error
}

func (m *mockUserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return m.createUserAndIdentityFunc(ctx, user, identity)
}

func (m *mockUserRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	return m.findIdentityByKeyFunc(ctx, provider, key)
}

func (m *mockUserRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return m.findUserByIDFunc(ctx, userID)
}

func (m *mockUserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	if m.updateUserFunc != nil {
		return m.updateUserFunc(ctx, userID, updates)
	}
	return nil
}

func TestSignup_Success(t *testing.T) {
	callCount := 0
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			callCount++
			// First call: checking if email already exists (should return not found)
			if callCount == 1 {
				return nil, errors.New("identity not found")
			}
			// Second call: retrieving the created user (should return the identity)
			return &model.UserAuthIdentity{
				IdentityID:  1,
				UserID:      1,
				Provider:    provider,
				ProviderKey: key,
			}, nil
		},
		createUserAndIdentityFunc: func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
			return nil
		},
	}

	cfg := &config.Config{
		JWTKey: "test-secret-key-32-characters-long",
	}

	service := NewAuthService(mockRepo, cfg)

	userID, err := service.Signup(context.Background(), "email", "john@example.com", "Password123!", "client")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if userID == 0 {
		t.Errorf("Expected non-zero user ID, got %d", userID)
	}
}

func TestSignup_MissingFields(t *testing.T) {
	mockRepo := &mockUserRepo{}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	tests := []struct {
		name        string
		provider    string
		providerKey string
		password    string
		role        string
	}{
		{"Missing providerKey", "email", "", "Pass123!", "client"},
		{"Missing password", "email", "test@example.com", "", "client"},
		{"Missing role", "email", "test@example.com", "Pass123!", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Signup(context.Background(), tt.provider, tt.providerKey, tt.password, tt.role)
			if err == nil {
				t.Error("Expected error for missing fields, got nil")
			}
		})
	}
}

func TestSignup_InvalidProvider(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Signup(context.Background(), "invalid-provider", "test@example.com", "Pass123!", "client")
	if err == nil {
		t.Error("Expected error for invalid provider, got nil")
	}
}

func TestSignup_InvalidRole(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Signup(context.Background(), "email", "test@example.com", "Pass123!", "superadmin")
	if err == nil {
		t.Error("Expected error for invalid role, got nil")
	}
}

func TestSignup_InvalidEmail(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Signup(context.Background(), "email", "not-an-email", "Pass123!", "client")
	if err == nil {
		t.Error("Expected error for invalid email, got nil")
	}
}

func TestSignup_WeakPassword(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	tests := []struct {
		name     string
		password string
	}{
		{"Too short", "Pass1!"},
		{"No uppercase", "password123!"},
		{"No lowercase", "PASSWORD123!"},
		{"No digit", "Password!"},
		{"No special char", "Password123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Signup(context.Background(), "email", "test@example.com", tt.password, "client")
			if err == nil {
				t.Errorf("Expected error for weak password %s, got nil", tt.password)
			}
		})
	}
}

func TestSignup_DuplicateUser(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return &model.UserAuthIdentity{
				IdentityID:  1,
				UserID:      1,
				Provider:    provider,
				ProviderKey: key,
			}, nil
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Signup(context.Background(), "email", "existing@example.com", "Password123!", "client")
	if err == nil {
		t.Error("Expected error for duplicate user, got nil")
	}
}

func TestLogin_Success(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)

	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return &model.UserAuthIdentity{
				IdentityID:   1,
				UserID:       1,
				Provider:     "email",
				ProviderKey:  "test@example.com",
				PasswordHash: string(hashedPassword),
			}, nil
		},
		findUserByIDFunc: func(ctx context.Context, userID int) (*model.User, error) {
			return &model.User{
				UserID:   1,
				FullName: "John Doe",
				Role:     "client",
			}, nil
		},
	}

	cfg := &config.Config{
		JWTKey: "test-secret-key-32-characters-long",
	}

	service := NewAuthService(mockRepo, cfg)

	token, err := service.Login(context.Background(), "email", "test@example.com", "Password123!")
	if err != nil {
		t.Errorf("Expected successful login, got error: %v", err)
	}

	if token == "" {
		t.Error("Expected token, got empty string")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("CorrectPassword123!"), bcrypt.DefaultCost)

	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return &model.UserAuthIdentity{
				IdentityID:   1,
				UserID:       1,
				PasswordHash: string(hashedPassword),
			}, nil
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Login(context.Background(), "email", "test@example.com", "WrongPassword123!")
	if err == nil {
		t.Error("Expected error for wrong password, got nil")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Login(context.Background(), "email", "nonexistent@example.com", "Password123!")
	if err == nil {
		t.Error("Expected error for non-existent user, got nil")
	}
}

func TestParseToken_Success(t *testing.T) {
	cfg := &config.Config{
		JWTKey: "test-secret-key-32-characters-long",
	}

	service := NewAuthService(&mockUserRepo{}, cfg)

	// Create a valid token
	claims := &model.Claims{
		UserID: 1,
		Role:   "client",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(cfg.JWTKey))

	parsedClaims, err := service.ParseToken(context.Background(), tokenString)
	if err != nil {
		t.Errorf("Expected successful parse, got error: %v", err)
	}

	if parsedClaims == nil {
		t.Error("Expected claims, got nil")
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	cfg := &config.Config{
		JWTKey: "test-secret-key-32-characters-long",
	}

	service := NewAuthService(&mockUserRepo{}, cfg)

	// Create an expired token
	claims := &model.Claims{
		UserID: 1,
		Role:   "client",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(cfg.JWTKey))

	_, err := service.ParseToken(context.Background(), tokenString)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	cfg := &config.Config{
		JWTKey: "test-secret-key-32-characters-long",
	}

	service := NewAuthService(&mockUserRepo{}, cfg)

	_, err := service.ParseToken(context.Background(), "invalid.token.string")
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

func TestIsEmailValid(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name@domain.co.uk", true},
		{"user+tag@example.com", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"user@", false},
		{"", false},
		{"user@domain", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isEmailValid(tt.email)
			if result != tt.valid {
				t.Errorf("isEmailValid(%s) = %v, expected %v", tt.email, result, tt.valid)
			}
		})
	}
}
