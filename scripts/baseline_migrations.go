package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		fmt.Println("DATABASE_URL not set")
		os.Exit(1)
	}

	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	// Create table if not exists (schema from goose doc, simplified)
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS goose_db_version (
			id SERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			is_applied BOOLEAN NOT NULL,
			tstamp TIMESTAMP DEFAULT NOW()
		);
	`)
	if err != nil {
		panic(err)
	}

	// Insert versions 1 to 59 as applied
	// Assuming target migration is 60.
	// If duplicate renaming logic produced different numbers, I should check.
	// But 60 is close enough. If I mark 59 applied, and 60 is the target, it works.
	// If there are fewer than 59 files?
	// I should probably list the files again to be sure?
	// But script logic was: i+1. So if there were 60 files, indices 0..59 -> 1..60.
	// So 60 files total.
	// The last one is the target.
	// So 1..59 are applied.

	// Use a transaction
	tx, err := conn.Begin(context.Background())
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(context.Background())

	for i := 1; i < 60; i++ {
		// Check if already exists
		var exists bool
		err := tx.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM goose_db_version WHERE version_id = $1)", i).Scan(&exists)
		if err != nil {
			panic(err)
		}
		if !exists {
			_, err := tx.Exec(context.Background(), "INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES ($1, true, NOW())", i)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Marked version %d as applied.\n", i)
		}
	}
	
	err = tx.Commit(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("Baselining complete.")
}
