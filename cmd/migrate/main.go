package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fallback for local testing if env not set, matching auth_integration_test.go fallback
		dbURL = "postgresql://postgres:password@localhost:5432/relaxation_hub_test"
		fmt.Printf("DATABASE_URL not set, using default: %s\n", dbURL)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(ctx)

	// Read 001.sql
	// Assuming running from project root or finding relative to this file
	// Better to assume we run `go run cmd/migrate/main.go` from root
	// Default to 001.sql
	path := "internal/db/migrations/001.sql"
	
	// If argument provided, use that
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try absolute path resolution assuming standard layout if relative fails
		wd, _ := os.Getwd()
		// Only join if it's the default path or doesn't look absolute? 
		// Simpler: just check if the arg exists. If not, try inside internal/...
		if len(os.Args) <= 1 {
			path = filepath.Join(wd, "internal", "db", "migrations", "001.sql")
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Unable to read 001.sql: %v\n", err)
	}

	fmt.Println("Applying migrations from 001.sql...")
	_, err = conn.Exec(ctx, string(content))
	if err != nil {
		log.Fatalf("Failed to execute migration: %v\n", err)
	}

	fmt.Printf("Successfully applied %s migration.\n", path)
}
