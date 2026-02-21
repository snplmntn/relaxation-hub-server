package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookingRepository_CreateAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewBookingRepository(tx)
		ctx := context.Background()

		// Seed Client
		var clientID int64
		err := tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639000000001', 'client', 'Test Client') RETURNING user_id`).Scan(&clientID)
		require.NoError(t, err, "Failed to seed client")

		// Seed Address
		var addressID int64
		err = tx.QueryRow(ctx, `INSERT INTO addresses (user_id, label, street_address, city, latitude, longitude) VALUES ($1, 'Home', '123 Test St', 'Manila', 14.5, 121.0) RETURNING address_id`, clientID).Scan(&addressID)
		require.NoError(t, err, "Failed to seed address")

		// Seed Service
		var serviceID int64
		err = tx.QueryRow(ctx, `INSERT INTO services (name, base_price, duration_minutes, category) VALUES ('Massage', 500, 60, 'wellness') RETURNING service_id`).Scan(&serviceID)
		require.NoError(t, err, "Failed to seed service")

		// Create Booking
		scheduledStart := time.Now().Add(24 * time.Hour)
		booking := &model.Booking{
			ClientID:        clientID,
			ServiceID:       int64Ptr(serviceID),
			AddressID:       int64Ptr(addressID),
			PaymentMethod:   "cash",
			GenderPref:      "any",
			PressurePref:    "medium",
			Notes:           "Test booking",
			DurationMinutes: 60,
			ScheduledStart:  timePtr(scheduledStart),
			RawTotal:        float64Ptr(500),
			FinalTotal:      float64Ptr(500),
			Status:          "pending",
			ReferenceCode:   strPtr("REF-12345"),
		}

		err = repo.Create(ctx, booking)
		require.NoError(t, err)
		assert.NotZero(t, booking.BookingID)
		assert.NotZero(t, booking.CreatedAt)

		// Get Booking
		fetched, err := repo.GetByBookingID(ctx, booking.BookingID)
		require.NoError(t, err)
		assert.Equal(t, *booking.ReferenceCode, *fetched.ReferenceCode)
		assert.Equal(t, *booking.FinalTotal, *fetched.FinalTotal)
	})
}

func TestBookingRepository_AssignmentConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewBookingRepository(tx)
		ctx := context.Background()

		// Seed Data
		var clientID, therapistID int64
		_ = tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639000000002', 'client', 'Client 2') RETURNING user_id`).Scan(&clientID)
		_ = tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639000000003', 'therapist', 'Therapist 1') RETURNING user_id`).Scan(&therapistID)

		// Ensure Therapist Profile exists and accepts assignments
		_, err := tx.Exec(ctx, `INSERT INTO therapist_profiles (therapist_id, accept_assignments) VALUES ($1, true)`, therapistID)
		require.NoError(t, err)

		start := time.Now().Add(2 * time.Hour)
		// Create Booking
		booking := &model.Booking{
			ClientID:        clientID,
			PaymentMethod:   "cash",
			Status:          "pending",
			ReferenceCode:   strPtr("REF-RACE"),
			ScheduledStart:  timePtr(start),
			DurationMinutes: 60,
		}
		// Insert raw to skip constraints we might not populate in struct
		err = tx.QueryRow(ctx, `
			INSERT INTO bookings (client_id, payment_method, status, reference_code, scheduled_start, duration_minutes) 
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING booking_id`,
			booking.ClientID, booking.PaymentMethod, booking.Status, booking.ReferenceCode, booking.ScheduledStart, booking.DurationMinutes).Scan(&booking.BookingID)
		require.NoError(t, err)

		// Attempt Assignment
		err = repo.AssignTherapist(ctx, booking.BookingID, therapistID)
		require.NoError(t, err, "First assignment should succeed")

		// Attempt Re-assignment (Conflict)
		err = repo.AssignTherapist(ctx, booking.BookingID, therapistID)
		assert.Error(t, err, "Second assignment should fail")
	})
}
