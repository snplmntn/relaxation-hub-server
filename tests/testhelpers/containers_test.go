package testhelpers

import (
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

