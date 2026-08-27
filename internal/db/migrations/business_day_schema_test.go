package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func loadForwardMigrations(t *testing.T) string {
	t.Helper()

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

	return strings.Join(strings.Fields(strings.ToLower(sql.String())), " ")
}

func TestBusinessDayForwardMigrationCreatesFunction(t *testing.T) {
	normalized := loadForwardMigrations(t)

	for _, required := range []string{
		"create or replace function business_day(utc_ts timestamp)",
		"returns date",
		// Interval arithmetic on the stored UTC value, not AT TIME ZONE. Manila
		// is a fixed UTC+8 and the trading day rolls over at 04:00, so the rule
		// is (+8h -4h) = +4h. Keeping it to arithmetic is what makes the
		// function IMMUTABLE, and only an IMMUTABLE function can back an index.
		"select (utc_ts + interval '4 hours')::date",
		"immutable",
		"create index if not exists idx_bookings_business_day",
		"on bookings (business_day(scheduled_start))",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing business_day forward migration containing %q", required)
		}
	}
}

// The function is only worth having if it is the single definition. A query
// that reintroduces the old calendar-date-of-actual_end rule silently splits a
// night's takings across two business days again, which is exactly the bug the
// function exists to close.
func TestNoQueryReintroducesTheCalendarDayRule(t *testing.T) {
	roots := []string{"../../repository", "../../service"}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}

			content, err := os.ReadFile(root + "/" + name)
			if err != nil {
				t.Fatalf("read %s/%s: %v", root, name, err)
			}

			if strings.Contains(string(content), "AT TIME ZONE 'Asia/Manila'") {
				t.Errorf("%s/%s derives a business date with AT TIME ZONE 'Asia/Manila'; use business_day(scheduled_start)", root, name)
			}
		}
	}
}
