//go:build ignore

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

	// Check if table exists
	var tableExists bool
	err = conn.QueryRow(context.Background(), "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'goose_db_version')").Scan(&tableExists)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Check table error: %v\n", err)
		os.Exit(1)
	}
	if !tableExists {
		fmt.Println("goose_db_version table does not exist.")
		return
	}

	rows, err := conn.Query(context.Background(), "SELECT version_id, is_applied, tstamp FROM goose_db_version ORDER BY version_id")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Println("Applied Migrations:")
	for rows.Next() {
		var vid int64
		var applied bool
		var tstamp any // nullable
		if err := rows.Scan(&vid, &applied, &tstamp); err != nil {
			fmt.Println("Scan error:", err)
			continue
		}
		fmt.Printf("Version: %d, Applied: %v\n", vid, applied)
	}
}
