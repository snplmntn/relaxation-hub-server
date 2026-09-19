package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestServiceAreaAreaKeyForwardMigrationWidensLegacyColumns(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var sql strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		sql.Write(content)
		sql.WriteByte('\n')
	}

	normalized := strings.Join(strings.Fields(strings.ToLower(sql.String())), " ")
	for _, required := range []string{
		"alter table public.service_areas alter column area_key type text",
		"alter table public.area_coverage_requests alter column area_key type text",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing service-area key widening migration containing %q", required)
		}
	}
}
