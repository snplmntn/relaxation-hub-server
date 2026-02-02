package integration

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_TravelBufferHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := SetupTestDB(t)
	repo := repository.NewTherapistRepository(pool)
	
	ctx := context.Background()

	// 1. Setup Data
	// Create Client
	_, clientID, _ := createTestUser(t, pool, "client_travel@test.com", "client")
	
	// Create Therapist
	_, therapistID, _ := createTestUser(t, pool, "therapist_travel@test.com", "therapist")
	require.NoError(t, repo.CreateProfile(ctx, therapistID))
	
	// Activate Therapist
	// Create a Branch
	var branchID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO branches (branch_name, latitude, longitude)
		VALUES ('Travel Test Branch', 14.5547, 121.0244)
		RETURNING branch_id
	`).Scan(&branchID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		UPDATE therapist_profiles 
		SET is_verified = TRUE, accept_assignments = TRUE, branch_id = $2
		WHERE therapist_id = $1
	`, therapistID, branchID)
	require.NoError(t, err)

	// Create Service
	var serviceID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ('Travel Massage', 'Test', 1000, 60, 'massage')
		RETURNING service_id
	`).Scan(&serviceID)
	require.NoError(t, err)

	// Assign Service to Therapist
	err = repo.AddService(ctx, &model.TherapistService{
		TherapistID: therapistID,
		ServiceID:   serviceID,
	})
	require.NoError(t, err)

	// Define Locations
	latA, lngA := 14.5591, 121.0180 
	addrAID := createAddressWithCoords(t, pool, clientID, "Makati PBCom", latA, lngA)
    
    // Verify Address A
	var checkLat, checkLng float64
	err = pool.QueryRow(ctx, `SELECT latitude, longitude FROM addresses WHERE address_id = $1`, addrAID).Scan(&checkLat, &checkLng)
	require.NoError(t, err)
	t.Logf("Address A Coords: %f, %f", checkLat, checkLng)
	assert.Equal(t, latA, checkLat)

	// Location B: One Ayala (Near, ~0.6km) -> Should have small buffer
	// latB, lngB := 14.5547, 121.0244
	// addrBID := createAddressWithCoords(t, pool, clientID, "One Ayala", latB, lngB)

	// Location C: BGC High Street (Far, ~3.5km) -> Should have ~26 min buffer
	latC, lngC := 14.5515, 121.0500
	// addrCID := createAddressWithCoords(t, pool, clientID, "BGC High Street", latC, lngC)

	// 2. Create Existing Booking at Location A (10:00 - 11:00)
	// We'll put it for Tomorrow 10:00 AM
	baseStart := time.Now().Add(24 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour).UTC() // Tomorrow 10:00 UTC
	// Ensure baseStart is distinct from other tests
	
	_, err = pool.Exec(ctx, `
		INSERT INTO bookings (
			client_id, service_id, therapist_id, address_id,
			status, scheduled_start, duration_minutes, 
			raw_total, final_total, created_at
		) VALUES ($1, $2, $3, $4, 'on_the_way', $5, 60, 1000, 1000, NOW())
	`, clientID, serviceID, therapistID, addrAID, baseStart)
	require.NoError(t, err)

	// 3. Test Cases for FindAvailableByServiceWithTime
	
	// Case A: Near Location (One Ayala), 11:15 AM (Gap 15 mins)
	// Buffer for 0.6km -> (0.6/20*60) + 15 = 1.8 + 15 = 16.8 mins.
	// Actually, wait. The SQL function logic:
	// IF distance < 0.5 THEN return 0
	// ELSE return CEIL(...)
	// 0.6 km is > 0.5km. So buffer is ~17 mins.
	// Gap is 11:00 to 11:15 = 15 mins.
	// 15 < 17. So this should FAIL (Conflict).
	// Let's adjust coordinate for "Walking Distance" (<0.5km).
	// One Ayala (14.5547, 121.0244) from PBCom (14.5591, 121.0180) is ~0.8km.
	// Let's use a closer point.
	// Location A2 (Very close): 14.5580, 121.0185 (~100m)
	latWalking, lngWalking := 14.5580, 121.0185
	
	t.Run("Walking Distance (<0.5km) - 15m Gap", func(t *testing.T) {
		// Try to book 11:15 - 12:15
		reqStart := baseStart.Add(75 * time.Minute) // 10:00 + 60m + 15m gap = 11:15
		
		res, err := repo.FindAvailableByServiceWithTime(
			ctx, clientID, serviceID, "any", "any", reqStart, 60, &latWalking, &lngWalking,
		)
		require.NoError(t, err)
		
		// Should find the therapist because buffer is 0 for <0.5km
		found := false
		for _, th := range res {
			if th.TherapistID == therapistID {
				found = true
				break
			}
		}
		assert.True(t, found, "Therapist should be available for walking distance with 15m gap")
	})

	t.Run("Far Distance (BGC) - 15m Gap", func(t *testing.T) {
		// Distance ~3.5km. Buffer ~26 mins.
		// Gap 15 mins. Should FAIL.
		reqStart := baseStart.Add(75 * time.Minute) // 11:15
		
		res, err := repo.FindAvailableByServiceWithTime(
			ctx, clientID, serviceID, "any", "any", reqStart, 60, &latC, &lngC,
		)
		require.NoError(t, err)
		
		found := false
		for _, th := range res {
			if th.TherapistID == therapistID {
				found = true
				break
			}
		}
		if found {
			t.Logf("Therapist WAS FOUND! This implies NO CONFLICT was detected.")
			t.Logf("Expected Conflict: Existing (10:00-11:00 @ Makati) vs Req (11:15-12:15 @ BGC). Dist=~3.6km. Buffer=~26m.")
			t.Logf("Existing End (11:00) + 26m = 11:26 > Req Start (11:15). Conflict SHOULD exist.")
		}
		assert.False(t, found, "Therapist should NOT be available for far distance with only 15m gap")
	})

	t.Run("Far Distance (BGC) - 45m Gap", func(t *testing.T) {
		// Distance ~3.5km. Buffer ~26 mins.
		// Gap 45 mins. Should SUCCEED.
		reqStart := baseStart.Add(105 * time.Minute) // 10:00 + 60m + 45m gap = 11:45
		
		res, err := repo.FindAvailableByServiceWithTime(
			ctx, clientID, serviceID, "any", "any", reqStart, 60, &latC, &lngC,
		)
		require.NoError(t, err)
		
		found := false
		for _, th := range res {
			if th.TherapistID == therapistID {
				found = true
				break
			}
		}
		assert.True(t, found, "Therapist SHOULD be available for far distance with 45m gap")
	})

}

func createAddressWithCoords(t *testing.T, pool interface{}, clientID int64, label string, lat, lng float64) int64 {
	var id int64
	p := pool.(db.DBTX)
	err := p.QueryRow(context.Background(), `
		INSERT INTO addresses (user_id, label, street_address, city, barangay, latitude, longitude)
		VALUES ($1, $2, 'Test Coords', 'TestCity', 'TestBrgy', $3, $4)
		RETURNING address_id
	`, clientID, label, lat, lng).Scan(&id)
	
	if err != nil {
		t.Fatalf("Failed to create address with coords: %v", err)
	}
	return id
}
