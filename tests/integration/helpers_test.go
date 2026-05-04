package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func int64Ptr(i int64) *int64        { return &i }
func float64Ptr(f float64) *float64  { return &f }
func strPtr(s string) *string        { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func init() {
	// Load .env file from project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	envPath := filepath.Join(projectRoot, ".env")

	// Ignore error if .env doesn't exist
	godotenv.Load(envPath)
}

// Test configuration - uses DATABASE_URL env var with fallback
func getTestDatabaseURL() string {
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	if dbURL := os.Getenv("TEST_DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	return ""
}

// getTestConfig returns config with current environment values
func getTestConfig() *config.Config {
	return &config.Config{
		DatabaseURL: getTestDatabaseURL(),
		JWTKey:      "test-secret-key-32-characters-long-for-testing",
		Port:        "8080",
	}
}

// SetupTestDB creates a test database pool using the centralized testhelpers
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	return testhelpers.SetupTestDB(t)
}

func patch(t *testing.T, pool *pgxpool.Pool, sql string) {
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Logf("Note: Patch %q failed (standard if already fixed): %v", sql, err)
	}
}

// CleanupTestDB removes test data
// Deprecated: Use transaction rollback instead.
func CleanupTestDB(t *testing.T, d db.DBTX) {}

func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}

	cfg := getTestConfig()
	ctx := context.Background()

	if pool, err := pgxpool.New(ctx, cfg.DatabaseURL); err == nil {
		defer pool.Close()
		fmt.Println("🧹 TestMain: Pre-run truncate DISABLED.")
		code := m.Run()
		fmt.Println("🧹 TestMain: Post-run truncate DISABLED.")
		os.Exit(code)
	} else {
		os.Exit(m.Run())
	}
}

// SetupTestRouter creates a test router with all routes
func SetupTestRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	userRepo := repository.NewUserRepository(d)
	referralRepo := repository.NewReferralRepository(d)

	authService := service.NewAuthService(userRepo, cfg)
	testRateLimitConfig := middleware.RateLimitConfig{
		MaxAttempts:     10000,
		LockoutDuration: 1 * time.Second,
		ResetWindow:     1 * time.Second,
		CheckInterval:   1 * time.Hour,
	}
	rateLimiter := middleware.NewRateLimiter(context.Background(), d, testRateLimitConfig)
	referralService := service.NewReferralService(referralRepo)
	authHandler := handler.NewAuthHandler(authService, rateLimiter, referralService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", authHandler.HandleSignup)
		r.Post("/login", authHandler.HandleLogin)

		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})
		})
	})

	return r
}

// Helper function to create a test user
func createTestUser(t *testing.T, d db.DBTX, emailBase, role string) (string, int64, string) {
	cfg := getTestConfig()
	router := SetupTestRouter(d, cfg)

	// Use unique email to avoid conflict if tests are run repeatedly
	email := uniqueTestEmail(emailBase)

	signupBody := map[string]string{
		"provider":     "email",
		"provider_key": email,
		"password":     "TestPassword123!",
		"role":         role,
	}

	body, _ := json.Marshal(signupBody)
	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Logf("Signup failed with status %d: %s", rr.Code, rr.Body.String())
	}

	var signupResp struct {
		Token  string `json:"token"`
		UserID int64  `json:"user_id"`
	}
	json.Unmarshal(rr.Body.Bytes(), &signupResp)

	// Fallback to query if signup didn't return UserID
	if signupResp.UserID == 0 {
		d.QueryRow(context.Background(), "SELECT user_id FROM users WHERE primary_email = $1", email).Scan(&signupResp.UserID)
	}

	// For non-client roles, we might need to login to get a token if signup didn't provide one
	token := signupResp.Token
	if token == "" {
		loginBody := map[string]string{
			"provider":     "email",
			"provider_key": email,
			"password":     "TestPassword123!",
		}
		body, _ = json.Marshal(loginBody)
		req = httptest.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("Login fallback failed with status %d: %s", rr.Code, rr.Body.String())
		}

		var loginResp struct {
			Token string `json:"token"`
		}
		json.Unmarshal(rr.Body.Bytes(), &loginResp)
		token = loginResp.Token
	}

	if role == "therapist" && signupResp.UserID != 0 {
		_, _ = d.Exec(context.Background(), `
			INSERT INTO therapist_profiles (therapist_id, is_verified, accept_assignments, at_branch)
			VALUES ($1, true, true, true)
			ON CONFLICT (therapist_id) DO NOTHING
		`, signupResp.UserID)
	}

	return token, signupResp.UserID, email
}

func uniqueTestEmail(base string) string {
	local := strings.TrimSpace(base)
	if at := strings.Index(local, "@"); at >= 0 {
		local = local[:at]
	}
	local = strings.ToLower(local)
	if local == "" {
		local = "user"
	}

	var b strings.Builder
	for _, r := range local {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == '+' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}

	safeLocal := b.String()
	if safeLocal == "" {
		safeLocal = "user"
	}
	return fmt.Sprintf("test_%d_%s@example.com", time.Now().UnixNano(), safeLocal)
}

func createTestAddress(t *testing.T, d db.DBTX, userID int64, token string, router *chi.Mux) string {
	if router != nil {
		addressBody := map[string]interface{}{
			"label":          "Home",
			"street_address": "123 Test St",
			"barangay":       "Test Barangay",
			"city":           "Test City",
			"province":       "Test Province",
			"postal_code":    "1234",
			"is_default":     true,
		}

		body, _ := json.Marshal(addressBody)
		req := httptest.NewRequest("POST", "/api/v1/addresses", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		if addressIDFloat, ok := response["address_id"].(float64); ok {
			return fmt.Sprintf("%d", int64(addressIDFloat))
		}
		if addressID, ok := response["address_id"].(string); ok {
			return addressID
		}
	} else {
		var addressID int64
		err := d.QueryRow(context.Background(), `
			INSERT INTO addresses (user_id, label, street_address, barangay, city, province, postal_code, is_default)
			VALUES ($1, 'Home', '123 Test St', 'Test Barangay', 'Test City', 'Test Province', '1234', true)
			RETURNING address_id
		`, userID).Scan(&addressID)
		if err == nil {
			return fmt.Sprintf("%d", addressID)
		}
	}
	return ""
}

func createTestService(t *testing.T, db db.DBTX) string {
	var serviceID string
	err := db.QueryRow(context.Background(), `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING service_id
	`, "Test Service", "Test Description", 1500.0, 60, "massage").Scan(&serviceID)

	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}

	return serviceID
}
