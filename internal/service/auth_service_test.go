package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// Mock UserRepository
type mockUserRepo struct {
	createUserAndIdentityFunc func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	createFunc                func(ctx context.Context, user *model.User) error
	findIdentityByKeyFunc     func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	findUserByIDFunc          func(ctx context.Context, userID int) (*model.User, error)
	updateUserFunc            func(ctx context.Context, userID int64, updates map[string]interface{}) error
	listUsersFunc             func(ctx context.Context, role string) ([]model.User, error)
	listUsersPaginatedFunc    func(ctx context.Context, role string, page, limit int, search string) ([]model.User, int, error)
}

func (m *mockUserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return m.createUserAndIdentityFunc(ctx, user, identity)
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, user)
	}
	user.UserID = 1 // Default mock ID
	return nil
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

func (m *mockUserRepo) ListUsers(ctx context.Context, role string) ([]model.User, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx, role)
	}
	return []model.User{}, nil
}

func (m *mockUserRepo) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}

func (m *mockUserRepo) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	return nil
}

func (m *mockUserRepo) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	return false, nil
}
func (m *mockUserRepo) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*repository.UserInfo, error) {
	return map[int64]*repository.UserInfo{}, nil
}
func (m *mockUserRepo) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*repository.TherapistInfo, error) {
	return map[int64]*repository.TherapistInfo{}, nil
}
func (m *mockUserRepo) GetBlockList(ctx context.Context, blockerID int64) ([]repository.BlockedUserEntry, error) {
	return []repository.BlockedUserEntry{}, nil
}
func (m *mockUserRepo) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	return nil
}
func (m *mockUserRepo) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	return nil, nil
}

func (m *mockUserRepo) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (m *mockUserRepo) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (m *mockUserRepo) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	return []model.User{}, nil
}

func (m *mockUserRepo) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return false, nil
}

func (m *mockUserRepo) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
<<<<<<< HEAD
func (m *mockUserRepo) SuspendUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (m *mockUserRepo) ListUsersPaginated(ctx context.Context, roleFilter string, page, limit int, search string) ([]model.User, int, error) {
	if m.listUsersPaginatedFunc != nil {
		return m.listUsersPaginatedFunc(ctx, roleFilter, page, limit, search)
	}
	return nil, 0, nil
}
=======
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

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

	userID, token, err := service.Signup(context.Background(), "email", "john@example.com", "Password123!", "client")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if userID == 0 {
		t.Errorf("Expected non-zero user ID, got %d", userID)
	}
	if token == "" {
		t.Errorf("Expected token for client signup, got empty string")
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
			_, _, err := service.Signup(context.Background(), tt.provider, tt.providerKey, tt.password, tt.role)
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

	_, _, err := service.Signup(context.Background(), "invalid-provider", "test@example.com", "Pass123!", "client")
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

	_, _, err := service.Signup(context.Background(), "email", "test@example.com", "Pass123!", "superadmin")
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

	_, _, err := service.Signup(context.Background(), "email", "not-an-email", "Pass123!", "client")
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
			_, _, err := service.Signup(context.Background(), "email", "test@example.com", tt.password, "client")
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

	_, _, err := service.Signup(context.Background(), "email", "existing@example.com", "Password123!", "client")
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
				UserID:        1,
				FullName:      "John Doe",
				Role:          "client",
				AccountStatus: "active",
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

// TestSignup_PhoneProvider tests phone number signup
func TestSignup_PhoneProvider(t *testing.T) {
	callCount := 0
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("identity not found")
			}
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

	userID, token, err := service.Signup(context.Background(), "phone", "+639123456789", "Password123!", "rider")
	if err != nil {
		t.Errorf("Expected no error for phone signup, got %v", err)
	}
	if userID == 0 {
		t.Errorf("Expected non-zero user ID, got %d", userID)
	}
	if token == "" {
		t.Errorf("Expected token for rider signup, got empty string")
	}
}

// TestSignup_InvalidPhoneNumber tests various invalid phone formats
func TestSignup_InvalidPhoneNumber(t *testing.T) {
	tests := []struct {
		name  string
		phone string
	}{
		{"Missing country code", "9123456789"},
		{"Wrong country code", "+631234567890"},
		{"Too short", "+6391234567"},
		{"Too long", "+63912345678901"},
		{"Invalid prefix", "+638123456789"},
		{"Contains letters", "+639abcdefghi"},
		{"Contains spaces", "+639 123 456 789"},
		{"With dashes", "+639-123-456-789"},
	}

	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return nil, errors.New("identity not found")
		},
	}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := service.Signup(context.Background(), "phone", tt.phone, "Password123!", "client")
			if err == nil {
				t.Errorf("Expected error for invalid phone %s, got nil", tt.phone)
			}
			if err != nil && err.Error() != "please input a valid phone number (+639xxxxxxxxx)" {
				t.Errorf("Expected phone validation error, got: %v", err)
			}
		})
	}
}

// TestSignup_DuplicatePhoneNumber tests duplicate phone registration
func TestSignup_DuplicatePhoneNumber(t *testing.T) {
	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return &model.UserAuthIdentity{
				IdentityID:  1,
				UserID:      1,
				Provider:    "phone",
				ProviderKey: "+639123456789",
			}, nil
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, _, err := service.Signup(context.Background(), "phone", "+639123456789", "Password123!", "client")
	if err == nil {
		t.Error("Expected error for duplicate phone number, got nil")
	}
	if err != nil && err.Error() != "phone number already in use" {
		t.Errorf("Expected 'phone number already in use', got: %v", err)
	}
}

// TestSignup_AdminAndTherapistNoToken tests that admin and therapist don't get tokens
func TestSignup_AdminAndTherapistNoToken(t *testing.T) {
	tests := []struct {
		role        string
		expectToken bool
	}{
		{"client", true},
		{"rider", true},
		{"admin", false},
		{"therapist", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			callCount := 0
			mockRepo := &mockUserRepo{
				findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
					callCount++
					if callCount == 1 {
						return nil, errors.New("identity not found")
					}
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

			cfg := &config.Config{JWTKey: "test-secret-key-32-characters-long"}
			service := NewAuthService(mockRepo, cfg)

			_, token, err := service.Signup(context.Background(), "email", "test@example.com", "Password123!", tt.role)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if tt.expectToken && token == "" {
				t.Errorf("Expected token for %s, got empty string", tt.role)
			}
			if !tt.expectToken && token != "" {
				t.Errorf("Expected no token for %s, got: %s", tt.role, token)
			}
		})
	}
}

// TestSignup_DatabaseErrors tests database error handling
func TestSignup_DatabaseErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mockUserRepo)
		expectedError string
	}{
		{
			name: "CreateUserAndIdentity fails",
			setupMock: func(m *mockUserRepo) {
				m.findIdentityByKeyFunc = func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
					return nil, errors.New("not found")
				}
				m.createUserAndIdentityFunc = func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
					return errors.New("database connection error")
				}
			},
			expectedError: "failed to create user",
		},
		{
			name: "FindIdentityByKey fails on retrieval",
			setupMock: func(m *mockUserRepo) {
				callCount := 0
				m.findIdentityByKeyFunc = func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
					callCount++
					if callCount == 1 {
						return nil, errors.New("not found")
					}
					return nil, errors.New("database error")
				}
				m.createUserAndIdentityFunc = func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
					return nil
				}
			},
			expectedError: "failed to retrieve created user",
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &mockUserRepo{}
			tt.setupMock(mockRepo)
			service := NewAuthService(mockRepo, cfg)

			_, _, err := service.Signup(context.Background(), "email", "test@example.com", "Password123!", "client")
			if err == nil {
				t.Error("Expected database error, got nil")
			}
			if err != nil && !contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}
		})
	}
}

// TestLogin_AccountStatusScenarios tests all account status variations
func TestLogin_AccountStatusScenarios(t *testing.T) {
	tests := []struct {
		name          string
		accountStatus string
		expectedError string
	}{
		{"active account", "active", ""},
		{"banned account", "banned", "Account is banned"},
		{"suspended account", "suspended", "Account is suspended"},
		{"inactive account", "inactive", "Account is inactive"},
		{"unknown status", "pending", "Account is not active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
						UserID:        1,
						Role:          "client",
						AccountStatus: tt.accountStatus,
					}, nil
				},
			}

			cfg := &config.Config{JWTKey: "test-secret-key-32-characters-long"}
			service := NewAuthService(mockRepo, cfg)

			token, err := service.Login(context.Background(), "email", "test@example.com", "Password123!")

			if tt.expectedError == "" {
				// Should succeed
				if err != nil {
					t.Errorf("Expected no error for active account, got: %v", err)
				}
				if token == "" {
					t.Error("Expected token for active account, got empty string")
				}
			} else {
				// Should fail with specific error
				if err == nil {
					t.Errorf("Expected error '%s', got nil", tt.expectedError)
				}
				if err != nil && err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// TestLogin_InvalidProvider tests login with unsupported providers
func TestLogin_InvalidProvider(t *testing.T) {
	tests := []struct {
		provider string
	}{
		{"facebook"},
		{"twitter"},
		{"github"},
		{"invalid"},
		{""},
	}

	mockRepo := &mockUserRepo{}
	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Parallel()
			_, err := service.Login(context.Background(), tt.provider, "test@example.com", "Password123!")
			if err == nil {
				t.Error("Expected error for invalid provider, got nil")
			}
			if err != nil && err.Error() != "unsupported provider" {
				t.Errorf("Expected 'unsupported provider', got: %v", err)
			}
		})
	}
}

// TestLogin_FindUserByIDError tests error handling when finding user by ID fails
func TestLogin_FindUserByIDError(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)

	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			return &model.UserAuthIdentity{
				IdentityID:   1,
				UserID:       1,
				PasswordHash: string(hashedPassword),
			}, nil
		},
		findUserByIDFunc: func(ctx context.Context, userID int) (*model.User, error) {
			return nil, errors.New("user not found in database")
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key"}
	service := NewAuthService(mockRepo, cfg)

	_, err := service.Login(context.Background(), "email", "test@example.com", "Password123!")
	if err == nil {
		t.Error("Expected error when FindUserByID fails, got nil")
	}
}

// TestLogin_PhoneProvider tests login with phone number
func TestLogin_PhoneProvider(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)

	mockRepo := &mockUserRepo{
		findIdentityByKeyFunc: func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
			if provider != "phone" {
				t.Errorf("Expected provider 'phone', got: %s", provider)
			}
			if key != "+639123456789" {
				t.Errorf("Expected key '+639123456789', got: %s", key)
			}
			return &model.UserAuthIdentity{
				IdentityID:   1,
				UserID:       1,
				Provider:     "phone",
				ProviderKey:  "+639123456789",
				PasswordHash: string(hashedPassword),
			}, nil
		},
		findUserByIDFunc: func(ctx context.Context, userID int) (*model.User, error) {
			return &model.User{
				UserID:        1,
				Role:          "rider",
				AccountStatus: "active",
			}, nil
		},
	}

	cfg := &config.Config{JWTKey: "test-secret-key-32-characters-long"}
	service := NewAuthService(mockRepo, cfg)

	token, err := service.Login(context.Background(), "phone", "+639123456789", "Password123!")
	if err != nil {
		t.Errorf("Expected successful login with phone, got error: %v", err)
	}
	if token == "" {
		t.Error("Expected token, got empty string")
	}
}

// TestParseToken_MalformedToken tests various malformed token formats
func TestParseToken_MalformedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"random string", "randomstring"},
		{"incomplete parts", "header.payload"},
		{"four parts", "header.payload.signature.extra"},
		{"invalid base64", "!!!.!!.!"},
	}

	cfg := &config.Config{JWTKey: "test-secret-key-32-characters-long"}
	service := NewAuthService(&mockUserRepo{}, cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := service.ParseToken(context.Background(), tt.token)
			if err == nil {
				t.Errorf("Expected error for malformed token '%s', got nil", tt.name)
			}
		})
	}
}

// TestParseToken_WrongSecret tests token with wrong signing key
func TestParseToken_WrongSecret(t *testing.T) {
	// Create token with different secret
	claims := &model.Claims{
		UserID: 1,
		Role:   "client",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("wrong-secret-key"))

	cfg := &config.Config{JWTKey: "correct-secret-key"}
	service := NewAuthService(&mockUserRepo{}, cfg)

	_, err := service.ParseToken(context.Background(), tokenString)
	if err == nil {
		t.Error("Expected error for token with wrong secret, got nil")
	}
}

// TestIsEmailValid tests email validation edge cases
func TestIsEmailValid(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name@domain.co.uk", true},
		{"user+tag@example.com", true},
		{"email@subdomain.example.com", true},
		{"firstname.lastname@example.com", true},
		{"email@123.123.123.123", true},
		{"1234567890@example.com", true},
		{"_______@example.com", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"user@", false},
		{"", false},
		{"user@domain", false},
		{"user@.com", false},
		{"user name@example.com", false},
		{"user@domain,com", false},
		{"user@@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			t.Parallel()
			result := isEmailValid(tt.email)
			if result != tt.valid {
				t.Errorf("isEmailValid(%s) = %v, expected %v", tt.email, result, tt.valid)
			}
		})
	}
}

// TestIsPhoneValid tests phone validation
func TestIsPhoneValid(t *testing.T) {
	tests := []struct {
		phone string
		valid bool
	}{
		{"+639123456789", true},
		{"+639987654321", true},
		{"+639000000000", true},
		{"+639999999999", true},
		{"639123456789", false},
		{"+631234567890", false},
		{"+6391234567", false},
		{"+63912345678901", false},
		{"+638123456789", false},
		{"+639 123 456 789", false},
		{"+639-123-456-789", false},
		{"", false},
		{"invalidphone", false},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			t.Parallel()
			result := isPhoneValid(tt.phone)
			if result != tt.valid {
				t.Errorf("isPhoneValid(%s) = %v, expected %v", tt.phone, result, tt.valid)
			}
		})
	}
}
