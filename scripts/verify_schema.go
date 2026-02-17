package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	// Simple check: try to select the new column
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		// Fallback for manual run if env not set
		// But in Makefile we export it.
		// For now, let's assume it's set or printed instructions
		fmt.Println("DATABASE_URL not set")
		os.Exit(1)
	}

	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var retryCount int
	// Try to query the new column. LIMIT 1 is enough.
	// If the column doesn't exist, this query will fail.
	err = conn.QueryRow(context.Background(), "SELECT retry_count FROM rides LIMIT 1").Scan(&retryCount)
	if err != nil {
		// It's okay if table is empty (Scan error), but if query preparation fails, err will be non-nil and contain "column does not exist"
		// If rows are empty, Scan returns ErrNoRows, which means query succeeded (column exists).
		if err == pgx.ErrNoRows {
			fmt.Println("SUCCESS: Column 'retry_count' exists (table is empty or no rows returned).")
			return
		}
		fmt.Fprintf(os.Stderr, "FAILURE: Query failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SUCCESS: Column 'retry_count' exists and was queried successfully.")
}
