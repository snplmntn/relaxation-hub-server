package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

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
	// First try to use the main DATABASE_URL (for Supabase)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	// Fallback to local PostgreSQL for development
	return "postgresql://postgres:password@localhost:5432/relaxation_hub_test"
}

// getTestConfig returns config with current environment values
func getTestConfig() *config.Config {
	return &config.Config{
		DatabaseURL: getTestDatabaseURL(),
		JWTKey:      "test-secret-key-32-characters-long-for-testing",
		Port:        "8080",
	}
}

// SetupTestDB creates a test database pool
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	cfg := getTestConfig()
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Skipf("Cannot connect to test database: %v. Skipping integration tests.", err)
		return nil
	}

	// Ping to verify connection
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Cannot ping test database: %v. Skipping integration tests.", err)
		return nil
	}

	return pool
}

// CleanupTestDB removes test data
// Deprecated: Use transaction rollback instead.
func CleanupTestDB(t *testing.T, d db.DBTX) {
	// No-op or use truncate if really needed, but ideally we roll back transactions.
	// Leaving this here if manual cleanup is forced, but empty is safe if we use txs.
}

// SetupTestRouter creates a test router with all routes
func SetupTestRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Setup repositories
	userRepo := repository.NewUserRepository(d)
	referralRepo := repository.NewReferralRepository(d)

	// Setup services
	authService := service.NewAuthService(userRepo, cfg)
	rateLimiter := middleware.NewRateLimiter(d, middleware.DefaultRateLimitConfig())
	referralService := service.NewReferralService(referralRepo)

	// Setup handlers
	authHandler := handler.NewAuthHandler(authService, rateLimiter, referralService)

	// Setup routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", authHandler.HandleSignup)
		r.Post("/login", authHandler.HandleLogin)

		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			// Protected routes can be added here
		})
	})

	return r
}

// Helper function to create a test user and return JWT token
func createTestUser(t *testing.T, d db.DBTX, email, role string) string {
	cfg := getTestConfig()
	router := SetupTestRouter(d, cfg)

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
		t.Logf("Warning: Failed to create test user %s with role %s: status %d", email, role, rr.Code)
		// Don't fail, as some roles may have constraints
	}

	// Login to get token
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
		t.Fatalf("Failed to login test user: %d", rr.Code)
	}

	var loginResponse map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &loginResponse)

	return loginResponse["token"].(string)
}

func createTestAddress(t *testing.T, pool *pgxpool.Pool, token string, router *chi.Mux) string {
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

	if addressID, ok := response["address_id"].(string); ok {
		return addressID
	}

	t.Fatal("Failed to create test address")
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
