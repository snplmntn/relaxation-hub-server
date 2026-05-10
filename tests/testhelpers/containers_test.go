package testhelpers

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCoreTablesExistQuery_UsesCurrentSchema(t *testing.T) {
	query := coreTablesExistQuery()

	if strings.Contains(query, "table_schema = 'public'") {
		t.Fatalf("query must not hardcode public schema: %s", query)
	}

	if !strings.Contains(query, "table_schema = current_schema()") {
		t.Fatalf("query must check current schema: %s", query)
	}
}

func TestTruncateAll_RefusesWithoutExplicitUnsafeOptIn(t *testing.T) {
	t.Setenv("RH_ALLOW_TRUNCATE_ALL", "")

	err := TruncateAll(context.Background(), nil)
	if !errors.Is(err, ErrUnsafeTruncateAll) {
		t.Fatalf("expected ErrUnsafeTruncateAll, got %v", err)
	}
}

func TestCleanupTestData_RefusesWithoutExplicitUnsafeOptIn(t *testing.T) {
	t.Setenv("RH_ALLOW_CLEANUP_TEST_DATA", "")

	err := CleanupTestData(context.Background(), nil)
	if !errors.Is(err, ErrUnsafeCleanupTestData) {
		t.Fatalf("expected ErrUnsafeCleanupTestData, got %v", err)
	}
}
