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

	// Cleanup pool when test ends to prevent connection leaks (MaxClients error)
	t.Cleanup(pool.Close)

	// Quick-fix migration for missing columns in test DB (if migration 038 didn't run)
	ctx := context.Background()
	_, _ = pool.Exec(ctx, "SET TIME ZONE 'UTC'")
	_, _ = pool.Exec(ctx, "DELETE FROM auth_rate_limits") // Clear stale rate limits to prevent 429 in tests

	patch(t, pool, "ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS at_branch BOOLEAN DEFAULT FALSE")
	patch(t, pool, "ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS last_location_update TIMESTAMP")

	// Legacy name column cleanup
	patch(t, pool, "ALTER TABLE branches ALTER COLUMN name DROP NOT NULL")
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS branch_name VARCHAR(150)")
	patch(t, pool, "UPDATE branches SET branch_name = name WHERE branch_name IS NULL AND name IS NOT NULL")
	
	// Ensure migration 034 columns exist (fix for stale test DB)
	// We use DEFAULT '' for required string fields to prevent Scan errors on NULL
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS address_line VARCHAR(255) DEFAULT ''")
	patch(t, pool, "UPDATE branches SET address_line = '' WHERE address_line IS NULL")
	
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS barangay VARCHAR(100)")
	
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS city VARCHAR(100) DEFAULT ''")
	patch(t, pool, "UPDATE branches SET city = '' WHERE city IS NULL")

	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS province VARCHAR(100) DEFAULT ''")
	patch(t, pool, "UPDATE branches SET province = '' WHERE province IS NULL")

	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20)")
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS latitude NUMERIC(9,6)")
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6)")
	patch(t, pool, "ALTER TABLE branches ADD COLUMN IF NOT EXISTS contact_no VARCHAR(20)")

	// Patch: Ensure 038 migration columns exist (at_branch)
	patch(t, pool, "ALTER TABLE therapist_profiles ADD COLUMN IF NOT EXISTS at_branch BOOLEAN DEFAULT TRUE")

	// Patch: Ensure 037 migration functions exist (Dynamic Travel Buffer)
	patch(t, pool, `
CREATE OR REPLACE FUNCTION calculate_distance_km(lat1 float, lon1 float, lat2 float, lon2 float)
RETURNS float AS $$
DECLARE
    R float := 6371;
    dLat float;
    dLon float;
    a float;
    c float;
BEGIN
    IF lat1 IS NULL OR lon1 IS NULL OR lat2 IS NULL OR lon2 IS NULL THEN
        RETURN NULL;
    END IF;
    dLat := radians(lat2 - lat1);
    dLon := radians(lon2 - lon1);
    a := sin(dLat/2) * sin(dLat/2) +
         sin(dLon/2) * sin(dLon/2) * cos(radians(lat1)) * cos(radians(lat2));
    c := 2 * atan2(sqrt(a), sqrt(1-a));
    RETURN R * c;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
`)

	patch(t, pool, `
CREATE OR REPLACE FUNCTION calculate_travel_buffer_minutes(distance_km float)
RETURNS int AS $$
BEGIN
    IF distance_km IS NULL THEN
        RETURN 30; 
    END IF;
    IF distance_km < 0.5 THEN
        RETURN 0;
    END IF;
    RETURN CEIL((distance_km / 20.0 * 60) + 15)::int;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
`)


	return pool
}

func patch(t *testing.T, pool *pgxpool.Pool, sql string) {
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Logf("Note: Patch %q failed (standard if already fixed): %v", sql, err)
	} else {
		t.Logf("✓ Applied patch (len=%d)", len(sql))
	}
}

// CleanupTestDB removes test data
// Deprecated: Use transaction rollback instead.
func CleanupTestDB(t *testing.T, d db.DBTX) {
	// No-op or use truncate if really needed, but ideally we roll back transactions.
	// Leaving this here if manual cleanup is forced, but empty is safe if we use txs.
}

// TruncateAll wipes all data from public tables.
// Used by TestMain to start with a clean slate.
func TruncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	// 1. Get all table names in public schema
	rows, err := pool.Query(ctx, `
		SELECT tablename FROM pg_tables 
		WHERE schemaname = 'public' 
		AND tablename != 'schema_migrations'
	`)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		tables = append(tables, table)
	}

	if len(tables) == 0 {
		return nil
	}

	// 2. Truncate all tables with CASCADE
	// We do this in a single statement for efficiency
	query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", quoteTables(tables))
	if _, err := pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("truncate failed: %w", err)
	}

	return nil
}

func quoteTables(tables []string) string {
	quoted := ""
	for i, t := range tables {
		if i > 0 {
			quoted += ", "
		}
		quoted += fmt.Sprintf("\"%s\"", t)
	}
	return quoted
}

func TestMain(m *testing.M) {
	// Parse flags
	if !flag.Parsed() {
		flag.Parse()
	}

	// Create a temporary connection just for cleanup
	cfg := getTestConfig()
	ctx := context.Background()

    // Only attempt cleanup if we can connect
	if pool, err := pgxpool.New(ctx, cfg.DatabaseURL); err == nil {
		defer pool.Close()

        // 1. Pre-run cleanup (Disabled for now to protect Staging DB usage)
        fmt.Println("🧹 TestMain: Pre-run truncate DISABLED (User Request).")
        // if err := TruncateAll(ctx, pool); err != nil {
        //     fmt.Printf("⚠️ Warning: Pre-run truncate failed: %v\n", err)
        // }
        
        // 2. Run tests
        code := m.Run()

        // 3. Post-run cleanup (Optional)
        if os.Getenv("TEST_DB_CLEANUP") == "true" {
            fmt.Println("🧹 TestMain: Cleaning database after tests...")
             if err := TruncateAll(ctx, pool); err != nil {
                fmt.Printf("⚠️ Warning: Post-run truncate failed: %v\n", err)
            }
        }

        os.Exit(code)
	} else {
        // If we can't connect, just run tests (they might fail connecting later, or skip)
        // We let the individual tests handle connection failures via SetupTestDB
        os.Exit(m.Run())
    }
}

// SetupTestRouter creates a test router with all routes
func SetupTestRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Setup repositories
	userRepo := repository.NewUserRepository(d)
	referralRepo := repository.NewReferralRepository(d)

	// Setup services
	authService := service.NewAuthService(userRepo, cfg)
	// Use relaxed rate limits for testing to avoid 429 errors
	testRateLimitConfig := middleware.RateLimitConfig{
		MaxAttempts:     10000,
		LockoutDuration: 1 * time.Second,
		ResetWindow:     1 * time.Second,
		CheckInterval:   1 * time.Hour,
	}
	rateLimiter := middleware.NewRateLimiter(context.Background(), d, testRateLimitConfig)
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

// Helper function to create a test user and return JWT token, user ID, and email
func createTestUser(t *testing.T, d db.DBTX, emailBase, role string) (string, int64, string) {
	cfg := getTestConfig()
	router := SetupTestRouter(d, cfg)

	// Use unique email to avoid conflict if tests are run repeatedly
	email := fmt.Sprintf("test_%d_%s", time.Now().UnixNano(), emailBase)

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
	}

	// Try to get token/user_id from signup response first
	var signupResp struct {
		Token  string `json:"token"`
		UserID int64  `json:"user_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &signupResp); err == nil && signupResp.Token != "" && signupResp.UserID != 0 {
		return signupResp.Token, signupResp.UserID, email
	}

	// Fallback to Login if signup didn't return token (e.g. verification needed)
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

	var loginResp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(rr.Body.Bytes(), &loginResp)

	// If we still don't have UserID, query it from DB
	if signupResp.UserID == 0 {
		d.QueryRow(context.Background(), "SELECT user_id FROM users WHERE primary_email = $1", email).Scan(&signupResp.UserID)
	}

	if (role == "therapist" || role == "admin") && signupResp.UserID != 0 {
		// ensure therapist profile exists if role is therapist
		if role == "therapist" {
			_, _ = d.Exec(context.Background(), `
				INSERT INTO therapist_profiles (therapist_id, is_verified, accept_assignments, at_branch)
				VALUES ($1, true, true, true)
				ON CONFLICT (therapist_id) DO NOTHING
			`, signupResp.UserID)
		}
	}

	return loginResp.Token, signupResp.UserID, email
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
		// Direct DB insert
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
