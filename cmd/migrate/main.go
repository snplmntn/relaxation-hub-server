// Command migrate applies pending SQL migrations from internal/db/migrations,
// tracking applied files in public.app_migration_history. Mirrors
// scripts/push_migrations.ps1 but uses pgx so it works without psql installed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

func main() {
	dryRun := false
	only := ""
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--dry-run":
			dryRun = true
		case "--only":
			if i+1 < len(os.Args) {
				only = os.Args[i+1]
				i++
			}
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	migrationsDir := filepath.Join("internal", "db", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if !dryRun {
		if _, err := pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS public.app_migration_history (
				filename TEXT PRIMARY KEY,
				applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);`); err != nil {
			log.Fatalf("ensure history table: %v", err)
		}
	}

	applied, skipped := 0, 0
	for _, name := range files {
		if only != "" && name != only {
			continue
		}
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM public.app_migration_history WHERE filename = $1)`,
			name,
		).Scan(&exists); err != nil {
			log.Fatalf("check history for %s: %v", name, err)
		}
		if exists {
			fmt.Printf("Skipping (already applied): %s\n", name)
			skipped++
			continue
		}

		if dryRun {
			fmt.Printf("Dry run - would apply: %s\n", name)
			applied++
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			log.Fatalf("read %s: %v", name, err)
		}

		fmt.Printf("Applying: %s\n", name)
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			log.Fatalf("apply %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO public.app_migration_history (filename) VALUES ($1) ON CONFLICT (filename) DO NOTHING`,
			name,
		); err != nil {
			log.Fatalf("record %s: %v", name, err)
		}
		applied++
	}

	fmt.Printf("Done. Applied: %d, Skipped: %d, Total: %d\n", applied, skipped, len(files))
}
