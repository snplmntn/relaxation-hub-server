package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// Mock AuthService
type mockAuthService struct {
	signupFunc     func(ctx context.Context, provider, providerKey, password, role string) (int, string, error)
	loginFunc      func(ctx context.Context, provider, providerKey, password string) (string, error)
	parseTokenFunc func(ctx context.Context, tokenString string) (jwt.Claims, error)
}

// Mock RateLimiter
type mockRateLimiter struct {
	isLockedFunc      func(identifier string) bool
	recordFailedFunc  func(identifier string) error
	resetAttemptsFunc func(identifier string) error
}

func (m *mockRateLimiter) IsLocked(identifier string) bool {
	if m.isLockedFunc != nil {
		return m.isLockedFunc(identifier)
	}
	return false
}

func (m *mockRateLimiter) RecordFailedAttempt(identifier string) error {
	if m.recordFailedFunc != nil {
		return m.recordFailedFunc(identifier)
	}
	return nil
}

func (m *mockRateLimiter) ResetAttempts(identifier string) error {
	if m.resetAttemptsFunc != nil {
		return m.resetAttemptsFunc(identifier)
	}
	return nil
}

func (m *mockAuthService) Signup(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
	return m.signupFunc(ctx, provider, providerKey, password, role)
}

func (m *mockAuthService) Login(ctx context.Context, provider, providerKey, password string) (string, error) {
	return m.loginFunc(ctx, provider, providerKey, password)
}

func (m *mockAuthService) ParseToken(ctx context.Context, tokenString string) (jwt.Claims, error) {
	return m.parseTokenFunc(ctx, tokenString)
}

// Mock ReferralService
type mockReferralService struct {
	completeReferralByCodeFunc func(ctx context.Context, code string, referredID int64) error
}

func (m *mockReferralService) Create(ctx context.Context, referrerID int64, req *model.CreateReferralRequest) (*model.Referral, error) {
	return nil, nil
}

func (m *mockReferralService) GetByCode(ctx context.Context, code string) (*model.Referral, error) {
	return nil, nil
}

func (m *mockReferralService) GetByReferrer(ctx context.Context, referrerID int64) ([]model.Referral, error) {
	return nil, nil
}

func (m *mockReferralService) CompleteReferral(ctx context.Context, referralID int64) error {
	return nil
}

func (m *mockReferralService) CompleteReferralByCode(ctx context.Context, code string, referredID int64) error {
	if m.completeReferralByCodeFunc != nil {
		return m.completeReferralByCodeFunc(ctx, code, referredID)
	}
	return nil
}

func (m *mockReferralService) GetRewardsByUser(ctx context.Context, userID int64) ([]model.ReferralReward, error) {
	return nil, nil
}

func (m *mockReferralService) RedeemReward(ctx context.Context, rewardID, userID int64) error {
	return nil
}

func TestHandleSignup_Success(t *testing.T) {
	mockService := &mockAuthService{
		signupFunc: func(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
			return 1, "valid.jwt.token", nil
		},
	}

	handler := NewAuthHandler(mockService, nil, nil)

	reqBody := AuthRequest{
		Provider:    "email",
		ProviderKey: "john@example.com",
		Password:    "Password123!",
		Role:        "client",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleSignup(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["token"] != "valid.jwt.token" {
		t.Errorf("Expected token in signup response, got %v", resp)
	}
}

func TestHandleSignup_InvalidJSON(t *testing.T) {
	mockService := &mockAuthService{}
	handler := NewAuthHandler(mockService, nil, nil)

	req := httptest.NewRequest("POST", "/register", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleSignup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleSignup_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		signupFunc: func(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
			return 0, "", errors.New("validation error")
		},
	}

	handler := NewAuthHandler(mockService, nil, nil)

	reqBody := AuthRequest{
		Provider:    "email",
		ProviderKey: "john@example.com",
		Password:    "Password123!",
		Role:        "client",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleSignup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleSignup_DuplicateEmail(t *testing.T) {
	mockService := &mockAuthService{
		signupFunc: func(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
			return 0, "", errors.New("email already in use")
		},
	}

	handler := NewAuthHandler(mockService, nil, nil)

	reqBody := AuthRequest{
		Provider:    "email",
		ProviderKey: "existing@example.com",
		Password:    "Password123!",
		Role:        "client",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleSignup(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", rr.Code)
	}
}

func TestHandleLogin_Success(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, provider, providerKey, password string) (string, error) {
			return "valid.jwt.token", nil
		},
	}

	handler := NewAuthHandler(mockService, nil, nil)

	reqBody := map[string]string{
		"provider":     "email",
		"provider_key": "john@example.com",
		"password":     "Password123!",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["token"] != "valid.jwt.token" {
		t.Errorf("Expected token in response, got %v", response)
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, provider, providerKey, password string) (string, error) {
			return "", errors.New("invalid credentials")
		},
	}

	handler := NewAuthHandler(mockService, nil, nil)

	reqBody := map[string]string{
		"provider":     "email",
		"provider_key": "john@example.com",
		"password":     "WrongPassword",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestHandleLogin_InvalidJSON(t *testing.T) {
	mockService := &mockAuthService{}
	handler := NewAuthHandler(mockService, nil, nil)

	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLogin(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleLogin_MissingFields(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, provider, providerKey, password string) (string, error) {
			if provider == "" || providerKey == "" || password == "" {
				return "", errors.New("missing required fields")
			}
			return "", nil
		},
	}
	handler := NewAuthHandler(mockService, nil, nil)

	tests := []struct {
		name    string
		reqBody map[string]string
	}{
		{"Missing provider", map[string]string{"provider_key": "test@example.com", "password": "Pass123!"}},
		{"Missing provider_key", map[string]string{"provider": "email", "password": "Pass123!"}},
		{"Missing password", map[string]string{"provider": "email", "provider_key": "test@example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.HandleLogin(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", rr.Code)
			}
		})
	}
}
