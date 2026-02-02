package integration

import (
	"context"
	"testing"
)

func TestMatchingLogic_DebugSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := SetupTestDB(t)
	ctx := context.Background()

    // Location A: 14.5547, 121.0244
    // Location B: 14.60, 121.03
	var dist float64
	err := pool.QueryRow(ctx, `SELECT calculate_distance_km(14.5547, 121.0244, 14.60, 121.03)`).Scan(&dist)
	if err != nil {
		t.Fatalf("SQL Func failed: %v", err)
	}
	t.Logf("Distance A->B: %f km", dist)
	
	var buffer int
	err = pool.QueryRow(ctx, `SELECT calculate_travel_buffer_minutes($1)`, dist).Scan(&buffer)
	if err != nil {
		t.Fatalf("Buffer Func failed: %v", err)
	}
	t.Logf("Buffer result: %d mins", buffer)

    if buffer < 15 {
        t.Errorf("Buffer %d is too small for distance %f!", buffer, dist)
    }
}
