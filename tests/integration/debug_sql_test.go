package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_DebugSQLFunctions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := SetupTestDB(t)
	ctx := context.Background()

	// Test Distance Calculation
	// PBCom (Makati) to High Street (BGC)
	// 14.5591, 121.0180 to 14.5515, 121.0500
	var dist float64
	err := pool.QueryRow(ctx, `SELECT calculate_distance_km(14.5591, 121.0180, 14.5515, 121.0500)`).Scan(&dist)
	require.NoError(t, err)
	t.Logf("Distance Calculated: %f km", dist)
	assert.Greater(t, dist, 3.0, "Distance should be > 3km")
	assert.Less(t, dist, 4.0, "Distance should be < 4km")

	// Test Buffer Calculation directly
	var buffer int
	err = pool.QueryRow(ctx, `SELECT calculate_travel_buffer_minutes(3.5)`).Scan(&buffer)
	require.NoError(t, err)
	t.Logf("Buffer Calculated for 3.5km: %d mins", buffer)
	// (3.5 / 20 * 60) + 15 = 10.5 + 15 = 25.5 -> 26
	assert.Equal(t, 26, buffer)
}
