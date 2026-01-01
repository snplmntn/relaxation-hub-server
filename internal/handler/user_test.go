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
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockUserService implements service.UserService minimally for handler tests.
type mockUserService struct {
	updateFunc func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error)
	getFunc    func(ctx context.Context, userID int64) (*model.User, error)
	listFunc   func(ctx context.Context, role string) ([]model.User, error)
	blockFunc  func(ctx context.Context, blockerID, blockedID int64) error
	unblockFunc func(ctx context.Context, blockerID, blockedID int64) error
}

func (m *mockUserService) Update(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, userID, updates)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) Get(ctx context.Context, userID int64) (*model.User, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) List(ctx context.Context, role string) ([]model.User, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, role)
	}
	return []model.User{}, nil
}

func (m *mockUserService) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	if m.blockFunc != nil {
		return m.blockFunc(ctx, blockerID, blockedID)
	}
	return nil
}

func (m *mockUserService) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	if m.unblockFunc != nil {
		return m.unblockFunc(ctx, blockerID, blockedID)
	}
	return nil
}
func (m *mockUserService) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error) {
	return []repository.BlockedUserEntry{}, nil
}
func (m *mockUserService) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	return nil
}

func (m *mockUserService) AddFavorite(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (m *mockUserService) RemoveFavorite(ctx context.Context, userID, therapistID int64) error {
	return nil
}

func (m *mockUserService) ListFavorites(ctx context.Context, userID int64) ([]model.User, error) {
	return []model.User{}, nil
}

func (m *mockUserService) IsFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return false, nil
}

func generateToken(t *testing.T, userID int64, role, key string) string {
	t.Helper()
	claims := &model.Claims{UserID: int(userID), Role: role}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(key))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestUpdateProfile_Success(t *testing.T) {
	jwtKey := "test-secret-key-32-char-value"

	mock := &mockUserService{
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			if updates["full_name"] != "Jane Doe" {
				return nil, errors.New("unexpected full_name")
			}
			return &model.User{UserID: int(userID), FullName: "Jane Doe", Gender: "female"}, nil
		},
	}

	handler := NewUserHandler(mock)
	h := middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey)

	body := map[string]string{"full_name": "Jane Doe", "gender": "female"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/profile", bytes.NewBuffer(b))
	token := generateToken(t, 42, "client", jwtKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var u model.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if u.FullName != "Jane Doe" {
		t.Fatalf("unexpected full_name: %s", u.FullName)
	}
}

func TestUpdateProfile_InvalidJSON(t *testing.T) {
	jwtKey := "test-secret-key-32-char-value"

	mock := &mockUserService{}
	handler := NewUserHandler(mock)
	h := middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey)

	req := httptest.NewRequest("PATCH", "/profile", bytes.NewBufferString("invalid json"))
	token := generateToken(t, 1, "client", jwtKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateProfile_NoFields(t *testing.T) {
	jwtKey := "test-secret-key-32-char-value"

	mock := &mockUserService{
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			return nil, errors.New("no fields to update")
		},
	}

	handler := NewUserHandler(mock)
	h := middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey)

	req := httptest.NewRequest("PATCH", "/profile", bytes.NewBufferString(`{}`))
	token := generateToken(t, 3, "client", jwtKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateProfile_Unauthorized(t *testing.T) {
	jwtKey := "test-secret-key-32-char-value"

	mock := &mockUserService{}
	handler := NewUserHandler(mock)
	h := middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateProfile), jwtKey)

	req := httptest.NewRequest("PATCH", "/profile", bytes.NewBufferString(`{"full_name":"X"}`))
	// no Authorization header
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestBlockUser_Success(t *testing.T) {
	jwtKey := "test-secret-key-32-char-value"

	mock := &mockUserService{
		blockFunc: func(ctx context.Context, blockerID, blockedID int64) error {
			if blockerID != 42 || blockedID != 99 {
				return errors.New("unexpected ids")
			}
			return nil
		},
	}

	handler := NewUserHandler(mock)
	h := middleware.AuthMiddleware(http.HandlerFunc(handler.BlockUser), jwtKey)

	body := map[string]int64{"blocked_user_id": 99}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/block", bytes.NewBuffer(b))
	token := generateToken(t, 42, "client", jwtKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}
