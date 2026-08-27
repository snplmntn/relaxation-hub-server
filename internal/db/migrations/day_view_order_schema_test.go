package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestDayViewOrderForwardMigrationCreatesOrderTable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var sql strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() == "001_init.sql" {
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
		"create table if not exists public.day_view_therapist_orders",
		"therapist_ids bigint[] not null default '{}'",
		"unique (view_key, business_date)",
		"create index if not exists idx_day_view_therapist_orders_view_date",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing day-view order forward migration containing %q", required)
		}
	}
}
