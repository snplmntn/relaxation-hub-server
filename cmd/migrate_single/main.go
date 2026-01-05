package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}
	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error connecting to DB: ", err)
	}
	defer pool.Close()

	migPath := filepath.Join("internal", "db", "migrations", "028_add_commission_columns.sql")
	content, err := os.ReadFile(migPath)
	if err != nil {
		log.Fatal("Failed to read migration file: ", err)
	}

	_, err = pool.Exec(context.Background(), string(content))
	if err != nil {
		log.Printf("Migration failed (might be already applied?): %v", err)
	} else {
		fmt.Println("Migration applied successfully.")
	}
}
