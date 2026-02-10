package integration

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestFoundation_DatabaseConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This spins up Postgres and applies all migrations
	pool := testhelpers.SetupTestDB(t)

	// Verify connection
	err := pool.Ping(context.Background())
	assert.NoError(t, err, "Database ping should succeed")

	// Verify Schema: Check if users table exists (from 001.sql)
	var tableName string
	err = pool.QueryRow(context.Background(), 
		"SELECT table_name FROM information_schema.tables WHERE table_name = 'users' AND table_schema = 'public'").Scan(&tableName)
	
	assert.NoError(t, err, "Users table query failed")
	assert.Equal(t, "users", tableName, "Users table should exist")

	// Verify Migrations: Check random late migration table (e.g., support_tickets from 016)
	err = pool.QueryRow(context.Background(), 
		"SELECT table_name FROM information_schema.tables WHERE table_name = 'support_tickets' AND table_schema = 'public'").Scan(&tableName)
	assert.NoError(t, err, "Support tickets table query failed")
	assert.Equal(t, "support_tickets", tableName, "Support tickets table should exist")
}
