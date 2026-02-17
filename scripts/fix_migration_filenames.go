package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

func main() {
	dir := "internal/db/migrations"
	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Identify the target file using partial match since exact name might change slightly if I mess up logic, 
	// but I want to track `add_ride_retry_columns.sql`
	targetSubstring := "add_ride_retry_columns"
	targetNewVersion := 0

	for i, oldName := range files {
		// New version is i+1
		newVersion := i + 1

		// Extract suffix (everything after the initial digits and optional underscore)
		// e.g. "001.sql" -> suffix ".sql" (name empty)
		// "002_foo.sql" -> suffix "foo.sql"
		
		var suffix string
		parts := strings.SplitN(oldName, "_", 2)
		if len(parts) == 2 {
			suffix = parts[1]
		} else {
			// No underscore. Check if it starts with digits.
			// "001.sql"
			rest := strings.TrimLeftFunc(oldName, unicode.IsDigit)
			if rest == "" || rest == ".sql" {
				suffix = "migration" + rest // Give it a default name if empty? "001_migration.sql"
                if rest == ".sql" {
                     suffix = "init.sql" // Guessing 001 is init
                }
			} else {
				suffix = rest
			}
		}

		newName := fmt.Sprintf("%03d_%s", newVersion, suffix)
		if oldName == newName {
			fmt.Printf("Skipping %s (already correct)\n", oldName)
			if strings.Contains(oldName, targetSubstring) {
				targetNewVersion = newVersion
			}
			continue
		}

		fmt.Printf("Renaming %s -> %s\n", oldName, newName)
		err := os.Rename(filepath.Join(dir, oldName), filepath.Join(dir, newName))
		if err != nil {
			panic(err)
		}
		
		if strings.Contains(oldName, targetSubstring) {
			targetNewVersion = newVersion
		}
	}

	fmt.Printf("\nTarget 'retry_columns' migration is now version: %d\n", targetNewVersion)
}
