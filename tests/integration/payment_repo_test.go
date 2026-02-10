package integration

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRepo_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewPaymentRepository(tx)
		ctx := context.Background()

		// Seed booking and admin user
		bookingID := seedBookingForPayment(t, tx)
		
		// Create admin user directly
		var adminID int64
		err := tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639333333333', 'admin', 'Admin User') RETURNING user_id`).Scan(&adminID)
		require.NoError(t, err)

		// 1. Create Payment
		payment := &model.Payment{
			BookingID: bookingID,
			Amount:    1000.0,
			Gateway:   "manual",
			Status:    "pending",
		}
		err = repo.Create(ctx, payment)
		require.NoError(t, err)
		assert.NotZero(t, payment.PaymentID)

		// 2. Update Proof
		proofURL := "http://example.com/proof.jpg"
		err = repo.UpdateProofURL(ctx, bookingID, proofURL)
		require.NoError(t, err)

		fetched, err := repo.GetByBookingID(ctx, bookingID)
		require.NoError(t, err)
		assert.Equal(t, proofURL, *fetched.ProofURL)

		// 3. Verify
		notes := "Verified manually"
		err = repo.Verify(ctx, bookingID, adminID, &notes)
		require.NoError(t, err)

		fetched, err = repo.GetByBookingID(ctx, bookingID)
		require.NoError(t, err)
		assert.Equal(t, "paid", fetched.Status)
		assert.NotZero(t, fetched.PaidAt)
		assert.Equal(t, notes, *fetched.Notes)
	})
}

func TestPaymentRepo_GetOrCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewPaymentRepository(tx)
		ctx := context.Background()

		bookingID := seedBookingForPayment(t, tx)

		// 1. First call - Creates
		p1, err := repo.GetOrCreateByBookingID(ctx, bookingID, 500.0, "gcash")
		require.NoError(t, err)
		assert.NotZero(t, p1.PaymentID)
		assert.Equal(t, "pending", p1.Status)

		// 2. Second call - Retrieves existing
		p2, err := repo.GetOrCreateByBookingID(ctx, bookingID, 500.0, "gcash")
		require.NoError(t, err)
		assert.Equal(t, p1.PaymentID, p2.PaymentID)
	})
}

func seedBookingForPayment(t *testing.T, tx pgx.Tx) int64 {
	ctx := context.Background()
	// Minimal seed to satisfy FKs
	var clientID int64
	err := tx.QueryRow(ctx, `INSERT INTO users (full_name, role, primary_phone) VALUES ('Payer', 'client', '+639444444444') RETURNING user_id`).Scan(&clientID)
	require.NoError(t, err, "Failed to seed client user")
	
	// Create minimal booking
	var bookingID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (client_id, payment_method, status, raw_total, final_total, duration_minutes) 
		VALUES ($1, 'cash', 'pending', 500, 500, 60) RETURNING booking_id`, 
		clientID).Scan(&bookingID)
	require.NoError(t, err, "Failed to seed booking")
	return bookingID
}
