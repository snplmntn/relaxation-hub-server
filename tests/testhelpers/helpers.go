package testhelpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrUnsafeCleanupTestData = errors.New("refusing to delete shared test data without RH_ALLOW_CLEANUP_TEST_DATA=1")
	ErrUnsafeTruncateAll     = errors.New("refusing to truncate all tables without RH_ALLOW_TRUNCATE_ALL=1")
)

// GenerateTestToken creates a JWT token for testing
func GenerateTestToken(userID int, role string, jwtKey string, duration time.Duration) (string, error) {
	claims := &model.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtKey))
}

// BeginTestTx starts a transaction for testing and returns a cleanup function that rolls it back.
func BeginTestTx(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, func(), error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = tx.Rollback(ctx)
	}
	return tx, cleanup, nil
}

// CreateTestUser creates a test user in the database
func CreateTestUser(ctx context.Context, d db.DBTX, fullName, email, role string) (int, error) {
	query := `
		INSERT INTO users(full_name, role, primary_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING user_id`

	var userID int
	err := d.QueryRow(ctx, query, fullName, role, email, time.Now(), time.Now()).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("failed to create test user: %w", err)
	}

	if role == "therapist" {
		_, _ = d.Exec(ctx, `
			INSERT INTO therapist_profiles (therapist_id, is_verified, accept_assignments)
			VALUES ($1, true, true)
			ON CONFLICT (therapist_id) DO NOTHING
		`, userID)
	}

	return userID, nil
}

// CreateTestUserWithPassword creates a test user with authentication identity
func CreateTestUserWithPassword(ctx context.Context, d db.DBTX, fullName, email, passwordHash, role string) (int, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		// If tx is a savepoint (pseudo-tx), rollback might be needed if commit wasn't called.
		// However, db.DBTX.Begin returns pgx.Tx which has Rollback.
		// If we commit successfully, Rollback does nothing.
		_ = tx.Rollback(ctx)
	}()

	// Create user
	query := `
		INSERT INTO users(full_name, role, primary_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING user_id`

	var userID int
	err = tx.QueryRow(ctx, query, fullName, role, email, time.Now(), time.Now()).Scan(&userID)
	if err != nil {
		return 0, err
	}

	// Create identity
	identityQuery := `
		INSERT INTO user_auth_identities(user_id, provider, provider_key, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err = tx.Exec(ctx, identityQuery, userID, "email", email, passwordHash, time.Now())
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}

	if role == "therapist" {
		_, _ = d.Exec(ctx, `
			INSERT INTO therapist_profiles (therapist_id, is_verified, accept_assignments)
			VALUES ($1, true, true)
			ON CONFLICT (therapist_id) DO NOTHING
		`, userID)
	}

	return userID, nil
}

// DeleteTestUser removes a test user from the database
func DeleteTestUser(ctx context.Context, d db.DBTX, userID int) error {
	// Delete identities first (foreign key constraint)
	_, err := d.Exec(ctx, "DELETE FROM user_auth_identities WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user identities: %w", err)
	}

	// Delete user
	_, err = d.Exec(ctx, "DELETE FROM users WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// CleanupTestData removes all test data from the database
// Deprecated: Use transaction-based testing (BeginTestTx) instead.
func CleanupTestData(ctx context.Context, d db.DBTX) error {
	if os.Getenv("RH_ALLOW_CLEANUP_TEST_DATA") != "1" {
		return ErrUnsafeCleanupTestData
	}

	tables := []string{
		"user_auth_identities",
		"users",
	}

	for _, table := range tables {
		_, err := d.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			return fmt.Errorf("failed to clean table %s: %w", table, err)
		}
	}

	return nil
}

// TestDatabaseAvailable checks if test database is available
func TestDatabaseAvailable(ctx context.Context, dbURL string) bool {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return false
	}
	defer pool.Close()

	return pool.Ping(ctx) == nil
}

// RandomEmail generates a random email for testing
func RandomEmail(prefix string) string {
	return fmt.Sprintf("%s_%d@test.example.com", prefix, time.Now().UnixNano())
}

// AssertNoError fails the test if error is not nil
func AssertNoError(t interface {
	Errorf(format string, args ...interface{})
}, err error, message string) {
	if err != nil {
		t.Errorf("%s: %v", message, err)
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t interface {
	Errorf(format string, args ...interface{})
}, expected, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t interface {
	Errorf(format string, args ...interface{})
}, value interface{}, message string) {
	if value == nil {
		t.Errorf("%s: expected non-nil value", message)
	}
}

// AssertNil fails the test if value is not nil
func AssertNil(t interface {
	Errorf(format string, args ...interface{})
}, value interface{}, message string) {
	if value != nil {
		t.Errorf("%s: expected nil value, got %v", message, value)
	}
}

// TruncateAll wipes all data from public tables.
func TruncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	if os.Getenv("RH_ALLOW_TRUNCATE_ALL") != "1" {
		return ErrUnsafeTruncateAll
	}

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
