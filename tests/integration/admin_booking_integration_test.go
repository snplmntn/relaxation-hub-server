package integration

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

// TestIntegration_AdminCreateBooking verifies admin can create booking via service
func TestIntegration_AdminCreateBooking(t *testing.T) {
    pool := SetupTestDB(t)
    if pool == nil {
        return
    }
    defer pool.Close()
    defer CleanupTestDB(t, pool)

    ctx := context.Background()
    // create admin and client users
    adminID, err := testhelpers.CreateTestUser(ctx, pool, "Admin User", "admin@test.com", "admin")
    if err != nil {
        t.Fatalf("failed to create admin: %v", err)
    }
    clientID, err := testhelpers.CreateTestUser(ctx, pool, "Client User", "client@test.com", "client")
    if err != nil {
        t.Fatalf("failed to create client: %v", err)
    }

    // create service (not required for this minimal flow); leave service/address nil
    _ = createTestService(t, pool)

    bookingRepo := repository.NewBookingRepository(pool)
    promotionRepo := repository.NewPromotionRepository(pool)
    assignmentQueueRepo := repository.NewAssignmentQueueRepository(pool)
    therapistRepo := repository.NewTherapistRepository(pool)
    offerRepo := repository.NewBookingOfferRepository(pool)
    bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, assignmentQueueRepo, therapistRepo, offerRepo)

    req := &model.CreateBookingRequest{
        DurationMinutes: 60,
        ScheduledStart:  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
        Notes:           "Admin created booking",
        GenderPref:      "any",
        PressurePref:    "medium",
    }

    b, err := bookingService.CreateForAdmin(ctx, int64(adminID), int64(clientID), req)
    if err != nil {
        t.Fatalf("CreateForAdmin failed: %v", err)
    }

    if b.ClientID != int64(clientID) {
        t.Fatalf("expected client_id %d, got %d", clientID, b.ClientID)
    }
    // ensure booking_id present
    if b.BookingID == 0 {
        t.Fatalf("expected booking_id to be set")
    }

    // verify timeline contains admin_created_booking (best-effort: read events)
    events, err := bookingRepo.ListEvents(ctx, b.BookingID)
    if err != nil {
        t.Fatalf("failed to list events: %v", err)
    }
    found := false
    for _, ev := range events {
        if ev.EventType == "admin_created_booking" {
            found = true
            break
        }
    }
    if !found {
        // fail the test to ensure audit logging is present
        t.Fatalf("admin_created_booking event not recorded; events: %v", events)
    }
}
