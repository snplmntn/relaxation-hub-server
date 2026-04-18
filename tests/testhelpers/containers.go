package testhelpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB creates a connection to the test database and applies migrations.
// It first checks for DATABASE_URL environment variable (e.g., for Supabase or existing DB).
// If DATABASE_URL is not set, it falls back to starting a Testcontainers Postgres instance.
//
// SOTA Practice: Flexible test configuration supporting both local containers and cloud databases.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

	// Check for existing database connection first
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		t.Logf("Using existing DATABASE_URL for tests")

		schemaName := os.Getenv("TEST_DB_SCHEMA")
		if schemaName == "" {
			schemaName = fmt.Sprintf("test_%d", time.Now().UnixNano())
		}

		cfg, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			t.Skipf("Cannot parse DATABASE_URL: %v. Skipping integration tests.", err)
			return nil
		}

		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", quoteIdent(schemaName)))
			if err != nil {
				return err
			}
			_, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", quoteIdent(schemaName)))
			return err
		}

		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Skipf("Cannot connect to DATABASE_URL: %v. Skipping integration tests.", err)
			return nil
		}

		if err := pool.Ping(ctx); err != nil {
			t.Skipf("Cannot ping DATABASE_URL: %v. Skipping integration tests.", err)
			return nil
		}

		t.Cleanup(pool.Close)
		t.Cleanup(func() {
			if err := dropSchema(context.Background(), pool, schemaName); err != nil {
				t.Logf("Warning: failed to drop test schema %s: %v", schemaName, err)
			}
		})

		// Apply migrations to ensure schema is up to date
		applyMigrations(t, pool)

		// SOTA Practice: Ensure consistent session state across all environments
		ctx := context.Background()
		_, _ = pool.Exec(ctx, "SET TIME ZONE 'UTC'")

		return pool
	}

	// Fallback to Testcontainers if DATABASE_URL not set
	t.Logf("DATABASE_URL not set, attempting to use Testcontainers...")

	dbName := "relaxation_hub_test"
	dbUser := "postgres"
	dbPassword := "password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Skipf("Cannot start Testcontainers (Docker may not be available): %v. Skipping integration tests.", err)
		return nil
	}

	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Warning: failed to terminate postgres container: %s", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create connection pool: %s", err)
	}

	t.Cleanup(pool.Close)
	t.Cleanup(func() {
		if err := TruncateAll(context.Background(), pool); err != nil {
			t.Logf("Warning: failed to truncate database during cleanup: %v", err)
		}
	})

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	applyMigrations(t, pool)

	// SOTA Practice: Ensure consistent session state
	_, _ = pool.Exec(ctx, "SET TIME ZONE 'UTC'")

	if err := TruncateAll(ctx, pool); err != nil {
		t.Logf("Warning: failed to truncate database: %v", err)
	}

	return pool
}

func dropSchema(ctx context.Context, pool *pgxpool.Pool, schemaName string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", quoteIdent(schemaName)))
	if err != nil && strings.Contains(err.Error(), "closed pool") {
		return nil
	}
	return err
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func coreTablesExistQuery() string {
	return `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema = current_schema()
		AND table_name IN ('users', 'bookings', 'payments')
	`
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	// SOTA Practice: Check if schema already exists before applying migrations
	// This allows tests to run against an already-migrated database (e.g., dev environment)
	var tableCount int
	err := pool.QueryRow(ctx, coreTablesExistQuery()).Scan(&tableCount)

	if err == nil && tableCount >= 3 {
		t.Logf("✓ Schema already exists (found %d core tables), skipping migrations", tableCount)
		return
	}

	// Schema doesn't exist, apply migrations
	t.Logf("Applying migrations to fresh database...")

	// Find migrations directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	var migrationsPath string

	for {
		candidate := filepath.Join(wd, "internal", "db", "migrations")
		if _, err := os.Stat(candidate); err == nil {
			migrationsPath = candidate
			break
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}

	if migrationsPath == "" {
		t.Fatal("could not find internal/db/migrations directory")
	}

	// Read all SQL files
	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	// Sort by name ensures 001 runs before 002
	var sqlFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		content, err := os.ReadFile(filepath.Join(migrationsPath, file))
		if err != nil {
			t.Fatalf("failed to read migration file %s: %v", file, err)
		}

		// SOTA: Remove BOM if present (common issue on Windows)
		if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
			t.Logf("Removing BOM from %s", file)
			content = content[3:]
		}

		t.Logf("Applying migration: %s", file)
		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			t.Fatalf("failed to execute migration %s: %v", file, err)
		}
	}
}
