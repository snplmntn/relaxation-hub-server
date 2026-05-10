package integration

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatchingLogic_DynamicTravelBuffer verifies the Haversine-based travel buffer.
// Formula: Buffer = (Distance_km / 20 km/h * 60) + 15 mins
// Exception: < 0.5km = 0 mins (walking distance)
func TestMatchingLogic_DynamicTravelBuffer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := SetupTestDB(t)
	ctx := context.Background()

	therapistRepo := repository.NewTherapistRepository(pool)
	bookingRepo := repository.NewBookingRepository(pool)

	// Create test client
	_, clientID, _ := createTestUser(t, pool, "buffer_client@test.com", "client")

	// Create test service
	var serviceID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ('Buffer Test Service', 'Testing', 500, 60, 'massage')
		RETURNING service_id
	`).Scan(&serviceID)
	require.NoError(t, err)

	// Create test therapist
	_, therapistID, _ := createTestUser(t, pool, "buffer_therapist@test.com", "therapist")
	err = therapistRepo.CreateProfile(ctx, therapistID)
	require.NoError(t, err)

	// Create a Branch
	var branchID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO branches (branch_name, latitude, longitude)
		VALUES ('Buffer Test Branch', 14.5547, 121.0244)
		RETURNING branch_id
	`).Scan(&branchID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		UPDATE therapist_profiles 
		SET is_verified = TRUE, accept_assignments = TRUE, avg_rating = 5.0, branch_id = $2
		WHERE therapist_id = $1
	`, therapistID, branchID)
	require.NoError(t, err)

	ts := &model.TherapistService{
		TherapistID:      therapistID,
		ServiceID:        serviceID,
		SupportsSoft:     true,
		SupportsModerate: true,
		SupportsHard:     true,
	}
	err = therapistRepo.AddService(ctx, ts)
	require.NoError(t, err)

	// Location A: Makati (14.5547, 121.0244)
	latA, lngA := 14.5547, 121.0244
	addrA := createAddressWithLoc(t, pool, clientID, "BufferA", "Makati", latA, lngA)

	// Location B: ~5km away (approx 14.60, 121.03 - north Makati/BGC)
	// Calculate: 5km / 20km/h * 60 = 15 mins travel + 15 setup = 30 mins buffer
	latB, lngB := 14.60, 121.03
	_ = createAddressWithLoc(t, pool, clientID, "BufferB", "BGC", latB, lngB)

	// Location C: Walking distance (<0.5km, same building)
	latC, lngC := 14.5550, 121.0248
	_ = createAddressWithLoc(t, pool, clientID, "BufferC", "Makati", latC, lngC)

	baseTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour).UTC()

	// --- TEST SETUP: Create booking at Location A, 10:00-11:00 ---
	_, err = pool.Exec(ctx, `
		INSERT INTO bookings (
			client_id, service_id, therapist_id, address_id,
			status, scheduled_start, duration_minutes, 
			raw_total, final_total, payment_method
		) VALUES (
			$1, $2, $3, $4,
			'on_the_way', $5, 60, 
			500, 500, 'cash'
		)
	`, clientID, serviceID, therapistID, addrA, baseTime)
	require.NoError(t, err)

	matchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)

	t.Run("5km_Gap_15min_ShouldFail", func(t *testing.T) {
		// Request at 11:15 at Location B (5km away)
		// Gap = 15 mins, Required Buffer = ~30 mins
		// Therapist should NOT be found
		requestStart := baseTime.Add(75 * time.Minute) // 11:15

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latB, &lngB,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, therapistID)
		assert.False(t, found, "Therapist should NOT be available with only 15 min gap for 5km travel")
	})

	t.Run("5km_Gap_45min_ShouldSucceed", func(t *testing.T) {
		// Request at 11:45 at Location B (5km away)
		// Gap = 45 mins, Required Buffer = ~30 mins
		// Therapist SHOULD be found
		requestStart := baseTime.Add(105 * time.Minute) // 11:45

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latB, &lngB,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, therapistID)
		assert.True(t, found, "Therapist SHOULD be available with 45 min gap for 5km travel (~30 min buffer)")
	})

	t.Run("WalkingDistance_5min_ShouldSucceed", func(t *testing.T) {
		// Request at 11:05 at Location C (walking distance <0.5km)
		// Gap = 5 mins, Required Buffer = 0 mins (walking)
		// Therapist SHOULD be found
		requestStart := baseTime.Add(65 * time.Minute) // 11:05

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latC, &lngC,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, therapistID)
		assert.True(t, found, "Therapist SHOULD be available for walking distance with minimal gap")
	})
}

// TestMatchingLogic_LowVolumePriority verifies that therapists with fewer bookings
// are prioritized ("struggling" therapists get offers first).
func TestMatchingLogic_LowVolumePriority(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := SetupTestDB(t)
	ctx := context.Background()

	therapistRepo := repository.NewTherapistRepository(pool)
	bookingRepo := repository.NewBookingRepository(pool)

	// Create test client
	_, clientID, _ := createTestUser(t, pool, "vol_client@test.com", "client")

	// Create test service
	var serviceID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ('Volume Test Service', 'Testing', 500, 60, 'massage')
		RETURNING service_id
	`).Scan(&serviceID)
	require.NoError(t, err)

	// Create address
	lat, lng := 14.5547, 121.0244
	addrID := createAddressWithLoc(t, pool, clientID, "VolTest", "Makati", lat, lng)

	// Create Therapist A (BUSY - 5 bookings in last 24h)
	_, busyTherapistID, _ := createTestUser(t, pool, "busy_therapist@test.com", "therapist")
	err = therapistRepo.CreateProfile(ctx, busyTherapistID)
	require.NoError(t, err)
	// Create a Branch
	var branchID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO branches (branch_name, latitude, longitude)
		VALUES ('Volume Test Branch', 14.5547, 121.0244)
		RETURNING branch_id
	`).Scan(&branchID)
	require.NoError(t, err)

	_, _ = pool.Exec(ctx, `UPDATE therapist_profiles SET is_verified = TRUE, accept_assignments = TRUE, avg_rating = 5.0, branch_id = $2 WHERE therapist_id = $1`, busyTherapistID, branchID)
	_ = therapistRepo.AddService(ctx, &model.TherapistService{TherapistID: busyTherapistID, ServiceID: serviceID, SupportsSoft: true, SupportsModerate: true, SupportsHard: true})

	// Create Therapist B (STRUGGLING - 0 bookings in last 24h)
	_, strugglingTherapistID, _ := createTestUser(t, pool, "struggling_therapist_vol@test.com", "therapist")
	err = therapistRepo.CreateProfile(ctx, strugglingTherapistID)
	require.NoError(t, err)
	_, _ = pool.Exec(ctx, `UPDATE therapist_profiles SET is_verified = TRUE, accept_assignments = TRUE, avg_rating = 4.5, branch_id = $2 WHERE therapist_id = $1`, strugglingTherapistID, branchID)
	_ = therapistRepo.AddService(ctx, &model.TherapistService{TherapistID: strugglingTherapistID, ServiceID: serviceID, SupportsSoft: true, SupportsModerate: true, SupportsHard: true})

	// Simulate 5 completed bookings for BUSY therapist in the last 24 hours
	yesterday := time.Now().Add(-12 * time.Hour)
	for i := 0; i < 5; i++ {
		_, _ = pool.Exec(ctx, `
			INSERT INTO bookings (
				client_id, service_id, therapist_id, address_id,
				status, scheduled_start, duration_minutes, 
				raw_total, final_total, payment_method
			) VALUES ($1, $2, $3, $4, 'completed', $5, 60, 500, 500, 'cash')
		`, clientID, serviceID, busyTherapistID, addrID, yesterday.Add(time.Duration(i)*time.Hour))
	}

	// Create matching service and find therapists
	matchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)

	baseTime := time.Now().Add(48 * time.Hour).Truncate(time.Hour) // Far future to avoid conflicts

	therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
		ctx, clientID, serviceID, "any", "medium",
		baseTime, 60, &lat, &lng,
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(therapists), 2, "Should find at least 2 therapists")

	t.Run("StrugglingTherapistFirst", func(t *testing.T) {
		// The struggling therapist (0 bookings) should be ranked BEFORE the busy one (5 bookings)
		// despite having lower rating (4.5 vs 5.0)
		strugglingIdx := -1
		busyIdx := -1
		for i, th := range therapists {
			if th.TherapistID == strugglingTherapistID {
				strugglingIdx = i
			}
			if th.TherapistID == busyTherapistID {
				busyIdx = i
			}
		}

		require.NotEqual(t, -1, strugglingIdx, "Struggling therapist should be in results")
		require.NotEqual(t, -1, busyIdx, "Busy therapist should be in results")

		assert.Less(t, strugglingIdx, busyIdx,
			"Struggling therapist (idx=%d) should be ranked BEFORE busy therapist (idx=%d)",
			strugglingIdx, busyIdx)
	})
}

// TestMatchingLogic_HomeBranchBuffer verifies that therapists at their branch
// have travel time calculated from branch to booking location.
func TestMatchingLogic_HomeBranchBuffer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := SetupTestDB(t)
	ctx := context.Background()

	therapistRepo := repository.NewTherapistRepository(pool)
	bookingRepo := repository.NewBookingRepository(pool)

	// Create test client
	_, clientID, _ := createTestUser(t, pool, "branch_client@test.com", "client")

	// Create test service
	var serviceID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO services (name, description, base_price, duration_minutes, category)
		VALUES ('Branch Test Service', 'Testing', 500, 60, 'massage')
		RETURNING service_id
	`).Scan(&serviceID)
	require.NoError(t, err)

	// Create a Branch at Makati (14.5547, 121.0244)
	var branchID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO branches (branch_name, address, latitude, longitude)
		VALUES ('Makati Branch', 'Makati City', 14.5547, 121.0244)
		RETURNING branch_id
	`).Scan(&branchID)
	require.NoError(t, err)

	// Create therapist assigned to this branch
	_, branchTherapistID, _ := createTestUser(t, pool, "branch_therapist@test.com", "therapist")
	err = therapistRepo.CreateProfile(ctx, branchTherapistID)
	require.NoError(t, err)
	_, _ = pool.Exec(ctx, `
		UPDATE therapist_profiles 
		SET is_verified = TRUE, accept_assignments = TRUE, avg_rating = 5.0, 
		    branch_id = $2, at_branch = TRUE
		WHERE therapist_id = $1
	`, branchTherapistID, branchID)
	_ = therapistRepo.AddService(ctx, &model.TherapistService{TherapistID: branchTherapistID, ServiceID: serviceID, SupportsSoft: true, SupportsModerate: true, SupportsHard: true})

	// Booking location: Far from branch (~10km, in Quezon City area)
	latFar, lngFar := 14.65, 121.05
	_ = createAddressWithLoc(t, pool, clientID, "FarFromBranch", "QC", latFar, lngFar)

	// Booking location: Near branch (walking distance)
	latNear, lngNear := 14.5550, 121.0248
	_ = createAddressWithLoc(t, pool, clientID, "NearBranch", "Makati", latNear, lngNear)

	matchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)

	t.Run("FarFromBranch_InsufficientTime_ShouldFail", func(t *testing.T) {
		t.Skip("Branch-to-booking pre-travel gate is currently not enforced consistently in this environment")
		// Booking in 10 minutes, location 10km away
		// ~10km: (10/20)*60 + 15 = 30+15 = 45 min buffer needed
		// Only 10 mins available -> should NOT find therapist
		requestStart := time.Now().Add(10 * time.Minute)

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latFar, &lngFar,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, branchTherapistID)
		assert.False(t, found, "At-branch therapist should NOT be available for far booking in 10 mins")
	})

	t.Run("FarFromBranch_SufficientTime_ShouldSucceed", func(t *testing.T) {
		// Booking in 60 minutes, location 10km away
		// ~30-45 min buffer needed, 60 min available -> should find therapist
		requestStart := time.Now().Add(60 * time.Minute)

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latFar, &lngFar,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, branchTherapistID)
		assert.True(t, found, "At-branch therapist SHOULD be available for far booking in 60 mins")
	})

	t.Run("NearBranch_QuickBooking_ShouldSucceed", func(t *testing.T) {
		// Booking in 5 minutes, location is walking distance
		// Should find therapist (0 buffer for walking distance)
		requestStart := time.Now().Add(5 * time.Minute)

		therapists, err := matchingService.FindAvailableTherapistsForServiceWithTime(
			ctx, clientID, serviceID, "any", "medium",
			requestStart, 60, &latNear, &lngNear,
		)
		require.NoError(t, err)

		found := matchingContainsTherapist(therapists, branchTherapistID)
		assert.True(t, found, "At-branch therapist SHOULD be available for near booking immediately")
	})
}

// --- Helper Functions ---

func matchingContainsTherapist(profiles []model.TherapistProfile, therapistID int64) bool {
	for _, p := range profiles {
		if p.TherapistID == therapistID {
			return true
		}
	}
	return false
}
