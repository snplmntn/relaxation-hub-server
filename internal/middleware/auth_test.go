package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeAccountStatusUserStore struct {
	user *model.User
	err  error
}

type countingAccountStatusUserStore struct {
	lookups int
}

func (f *countingAccountStatusUserStore) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	f.lookups++
	return &model.User{UserID: userID, AccountStatus: "active"}, nil
}

func (f fakeAccountStatusUserStore) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return f.user, f.err
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	jwtKey := "test-secret-key-32-characters-long"

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
	tokenString, _ := token.SignedString([]byte(jwtKey))

	// Create test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with auth middleware
	handler := AuthMiddleware(nextHandler, jwtKey)

	// Create request with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	jwtKey := "test-secret-key"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthMiddleware(nextHandler, jwtKey)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtKey := "test-secret-key"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthMiddleware(nextHandler, jwtKey)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.string")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	jwtKey := "test-secret-key-32-characters-long"

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
	tokenString, _ := token.SignedString([]byte(jwtKey))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthMiddleware(nextHandler, jwtKey)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MalformedAuthHeader(t *testing.T) {
	jwtKey := "test-secret-key"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthMiddleware(nextHandler, jwtKey)

	tests := []struct {
		name   string
		header string
	}{
		{"No Bearer prefix", "token123"},
		{"Only Bearer", "Bearer"},
		{"Extra spaces", "Bearer  token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.header)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", rr.Code)
			}
		})
	}
}

func TestAccountStatusMiddleware_AllowsActiveAccount(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := AccountStatusMiddleware(fakeAccountStatusUserStore{
		user: &model.User{UserID: 1, AccountStatus: "active"},
	}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, 1))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestAccountStatusMiddleware_CachesSuccessfulLookup(t *testing.T) {
	store := &countingAccountStatusUserStore{}
	handler := NewAccountStatusMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range 2 {
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, 1))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rr.Code)
		}
	}

	if store.lookups != 1 {
		t.Fatalf("FindUserByID calls = %d, want 1", store.lookups)
	}
}

func TestAccountStatusMiddleware_BlocksInactiveAccounts(t *testing.T) {
	for _, status := range []string{"inactive", "suspended", "blocked", "banned"} {
		t.Run(status, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler should not be called")
			})
			handler := AccountStatusMiddleware(fakeAccountStatusUserStore{
				user: &model.User{UserID: 1, AccountStatus: status},
			}, nextHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			req = req.WithContext(context.WithValue(req.Context(), userIDKey, 1))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Errorf("Expected status 403, got %d", rr.Code)
			}
		})
	}
}

func TestAccountStatusMiddleware_ReturnsUnauthorizedForMissingUser(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})
	handler := AccountStatusMiddleware(fakeAccountStatusUserStore{
		err: errors.New("user not found"),
	}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, 1))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestAccountStatusMiddleware_ReturnsUnavailableForLookupFailure(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})
	handler := AccountStatusMiddleware(fakeAccountStatusUserStore{
		err: errors.New("failed to find user: context deadline exceeded"),
	}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, 1))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}
}

func TestRoleMiddleware_ValidRole(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := RoleMiddleware([]string{"admin", "client"}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)

	// Set the role in context (normally done by AuthMiddleware)
	ctx := req.Context()
	ctx = SetUserRole(ctx, "admin")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRoleMiddleware_InvalidRole(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RoleMiddleware([]string{"admin"}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)

	// Set a different role
	ctx := req.Context()
	ctx = SetUserRole(ctx, "client")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestRoleMiddleware_MissingRole(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RoleMiddleware([]string{"admin"}, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}
}

func TestRoleGroups_AdminOperationsIncludeSuperAdmin(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RoleMiddleware(AdminOperationalRoles, nextHandler)

	for _, role := range []string{"admin", "super_admin"} {
		t.Run(role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req = req.WithContext(SetUserRole(req.Context(), role))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", role, rr.Code)
			}
		})
	}
}

func TestRoleGroups_SuperAdminOnlyExcludesAdmin(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RoleMiddleware(SuperAdminOnlyRoles, nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(SetUserRole(req.Context(), "admin"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for regular admin, got %d", rr.Code)
	}
}
