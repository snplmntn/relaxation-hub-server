package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockUserService implements service.UserService minimally for handler tests.
type mockUserService struct {
	updateFunc  func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error)
	getFunc     func(ctx context.Context, userID int64) (*model.User, error)
	listFunc    func(ctx context.Context, role string) ([]model.User, error)
	blockFunc   func(ctx context.Context, blockerID, blockedID int64) error
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
func (m *mockUserService) AdminBlockTherapistForClient(ctx context.Context, clientID, therapistID int64) error {
	return nil
}
func (m *mockUserService) AdminUnblockTherapistForClient(ctx context.Context, clientID, therapistID int64) error {
	return nil
}
func (m *mockUserService) AdminListClientBlocks(ctx context.Context, clientID int64) ([]repository.BlockedUserEntry, error) {
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

func (m *mockUserService) DeactivateClient(ctx context.Context, userID int64) (*model.User, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, userID, map[string]interface{}{"account_status": "inactive"})
	}
	return &model.User{UserID: int(userID), Role: model.RoleClient, AccountStatus: "inactive"}, nil
}

func (m *mockUserService) ReactivateClient(ctx context.Context, userID int64) (*model.User, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, userID, map[string]interface{}{"account_status": "active"})
	}
	return &model.User{UserID: int(userID), Role: model.RoleClient, AccountStatus: "active"}, nil
}

func (m *mockUserService) ListPaginated(ctx context.Context, role string, page, limit int, search string) ([]model.User, int, error) {
	if m.listFunc != nil {
		users, err := m.listFunc(ctx, role)
		return users, len(users), err
	}
	return []model.User{}, 0, nil
}

func (m *mockUserService) ListPaginatedFiltered(ctx context.Context, role, status string, vip *bool, page, limit int, search string) ([]model.User, int, error) {
	if m.listFunc != nil {
		users, err := m.listFunc(ctx, role)
		return users, len(users), err
	}
	return []model.User{}, 0, nil
}

func (m *mockUserService) CountByStatus(ctx context.Context, role, search string) (model.UserStatusCounts, error) {
	return model.UserStatusCounts{}, nil
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

func requestWithUserIDParam(req *http.Request, userID string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("userID", userID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
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

	handler := NewUserHandler(mock, nil, nil)
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
	handler := NewUserHandler(mock, nil, nil)
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

	handler := NewUserHandler(mock, nil, nil)
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
	handler := NewUserHandler(mock, nil, nil)
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

	handler := NewUserHandler(mock, nil, nil)
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

func TestAdminUpdateStatus_AllowsBlocked(t *testing.T) {
	mock := &mockUserService{
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			if userID != 42 {
				return nil, errors.New("unexpected user id")
			}
			if updates["account_status"] != "blocked" {
				return nil, errors.New("expected blocked status")
			}
			return &model.User{UserID: int(userID), Role: model.RoleClient, AccountStatus: "blocked"}, nil
		},
	}

	handler := NewUserHandler(mock, nil, nil)
	body := bytes.NewBufferString(`{"account_status":"blocked"}`)
	req := requestWithUserIDParam(httptest.NewRequest("PATCH", "/users/42/status", body), "42")
	rr := httptest.NewRecorder()

	handler.AdminUpdateStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	var user model.User
	if err := json.NewDecoder(rr.Body).Decode(&user); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if user.AccountStatus != "blocked" {
		t.Fatalf("expected blocked, got %q", user.AccountStatus)
	}
}

func TestAdminUpdateOperationalUserProfile_AllowsVIPToggle(t *testing.T) {
	mock := &mockUserService{
		getFunc: func(ctx context.Context, userID int64) (*model.User, error) {
			return &model.User{UserID: int(userID), Role: model.RoleClient, AccountStatus: "active"}, nil
		},
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			if userID != 42 {
				return nil, errors.New("unexpected user id")
			}
			if updates["is_vip"] != true {
				return nil, errors.New("expected is_vip true")
			}
			return &model.User{UserID: int(userID), Role: model.RoleClient, AccountStatus: "active", IsVIP: true}, nil
		},
	}

	handler := NewUserHandler(mock, nil, nil)
	body := bytes.NewBufferString(`{"is_vip":true}`)
	req := requestWithUserIDParam(httptest.NewRequest("PATCH", "/users/42", body), "42")
	rr := httptest.NewRecorder()

	handler.AdminUpdateOperationalUserProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	var user model.User
	if err := json.NewDecoder(rr.Body).Decode(&user); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !user.IsVIP {
		t.Fatal("expected VIP user in response")
	}
}

func TestListUsers_RejectsStaffRoleFilters(t *testing.T) {
	mock := &mockUserService{
		listFunc: func(ctx context.Context, role string) ([]model.User, error) {
			t.Fatalf("ListUsers must not query staff role %q through /users", role)
			return nil, nil
		},
	}
	handler := NewUserHandler(mock, nil, nil)
	h := http.HandlerFunc(handler.ListUsers)

	for _, role := range []string{"admin", "super_admin"} {
		t.Run(role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users?role="+role, nil)
			req = req.WithContext(middleware.SetUserRole(req.Context(), model.RoleAdmin))

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminExportUsers_RejectsStaffRoleFilters(t *testing.T) {
	mock := &mockUserService{
		listFunc: func(ctx context.Context, role string) ([]model.User, error) {
			t.Fatalf("AdminExportUsers must not query staff role %q through /users/export", role)
			return nil, nil
		},
	}
	handler := NewUserHandler(mock, nil, nil)
	h := http.HandlerFunc(handler.AdminExportUsers)

	for _, role := range []string{"admin", "super_admin"} {
		t.Run(role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users/export?role="+role, nil)
			req = req.WithContext(middleware.SetUserRole(req.Context(), model.RoleAdmin))

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminCreateUser_AllowsOnlyOperationalRoles(t *testing.T) {
	tests := []struct {
		role       string
		wantStatus int
	}{
		{role: "client", wantStatus: http.StatusCreated},
		{role: "therapist", wantStatus: http.StatusCreated},
		{role: "rider", wantStatus: http.StatusCreated},
		{role: "admin", wantStatus: http.StatusForbidden},
		{role: "super_admin", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			auth := &mockAuthService{
				signupFunc: func(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
					if role != tt.role {
						t.Fatalf("expected role %q, got %q", tt.role, role)
					}
					return 123, "", nil
				},
			}
			handler := NewUserHandler(&mockUserService{}, nil, auth)
			body := bytes.NewBufferString(`{"provider":"email","provider_key":"test@example.com","password":"Pass123!","role":"` + tt.role + `"}`)
			req := httptest.NewRequest("POST", "/users", body)
			rr := httptest.NewRecorder()

			handler.AdminCreateUser(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d. Body: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminCreateStaff_AllowsOnlyStaffRoles(t *testing.T) {
	tests := []struct {
		role       string
		wantStatus int
	}{
		{role: "admin", wantStatus: http.StatusCreated},
		{role: "super_admin", wantStatus: http.StatusCreated},
		{role: "client", wantStatus: http.StatusForbidden},
		{role: "therapist", wantStatus: http.StatusForbidden},
		{role: "rider", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			auth := &mockAuthService{
				signupStaffFunc: func(ctx context.Context, provider, providerKey, password, role string) (int, string, error) {
					if role != tt.role {
						t.Fatalf("expected role %q, got %q", tt.role, role)
					}
					return 456, "", nil
				},
			}
			handler := NewUserHandler(&mockUserService{}, nil, auth)
			body := bytes.NewBufferString(`{"provider":"email","provider_key":"staff@example.com","password":"Pass123!","role":"` + tt.role + `"}`)
			req := httptest.NewRequest("POST", "/staff", body)
			rr := httptest.NewRecorder()

			handler.AdminCreateStaff(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d. Body: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestListStaff_RespondsWithUsersKey(t *testing.T) {
	mock := &mockUserService{
		listFunc: func(ctx context.Context, role string) ([]model.User, error) {
			switch role {
			case model.RoleAdmin:
				return []model.User{{UserID: 1, Role: model.RoleAdmin, AccountStatus: "active"}}, nil
			case model.RoleSuperAdmin:
				return []model.User{{UserID: 2, Role: model.RoleSuperAdmin, AccountStatus: "active"}}, nil
			default:
				t.Fatalf("unexpected role %q", role)
				return nil, nil
			}
		},
	}
	handler := NewUserHandler(mock, nil, nil)
	req := httptest.NewRequest("GET", "/staff", nil)
	req = req.WithContext(middleware.SetUserRole(req.Context(), model.RoleSuperAdmin))
	rr := httptest.NewRecorder()

	handler.ListStaff(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["users"]; !ok {
		t.Fatalf("expected users key in staff response, got %#v", body)
	}
	if _, ok := body["staff"]; ok {
		t.Fatalf("staff response should not use legacy staff key: %#v", body)
	}
}

func TestAdminUpdateOperationalUserProfile_RejectsStaffTargets(t *testing.T) {
	mock := &mockUserService{
		getFunc: func(ctx context.Context, userID int64) (*model.User, error) {
			return &model.User{UserID: int(userID), Role: model.RoleSuperAdmin}, nil
		},
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			t.Fatalf("staff target must not be updated through /users")
			return nil, nil
		},
	}
	handler := NewUserHandler(mock, nil, nil)
	body := bytes.NewBufferString(`{"full_name":"Updated"}`)
	req := requestWithUserIDParam(httptest.NewRequest("PATCH", "/users/12", body), "12")
	rr := httptest.NewRecorder()

	handler.AdminUpdateOperationalUserProfile(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminUpdateStaffProfile_RejectsOperationalTargets(t *testing.T) {
	mock := &mockUserService{
		getFunc: func(ctx context.Context, userID int64) (*model.User, error) {
			return &model.User{UserID: int(userID), Role: model.RoleClient}, nil
		},
		updateFunc: func(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
			t.Fatalf("operational target must not be updated through /staff")
			return nil, nil
		},
	}
	handler := NewUserHandler(mock, nil, nil)
	body := bytes.NewBufferString(`{"full_name":"Updated"}`)
	req := requestWithUserIDParam(httptest.NewRequest("PATCH", "/staff/12", body), "12")
	rr := httptest.NewRecorder()

	handler.AdminUpdateStaffProfile(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}
