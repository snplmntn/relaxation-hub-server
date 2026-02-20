package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
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
    
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

    ctx := context.Background() 
	
    adminID, err := testhelpers.CreateTestUser(ctx, tx, "Admin User", testhelpers.RandomEmail("admin"), "admin")
    if err != nil {
        t.Fatalf("failed to create admin: %v", err)
    }
    clientID, err := testhelpers.CreateTestUser(ctx, tx, "Client User", testhelpers.RandomEmail("client"), "client")
    if err != nil {
        t.Fatalf("failed to create client: %v", err)
    }

    // Use d (tx) for repositories
    d := db.DBTX(tx)
    bookingRepo := repository.NewBookingRepository(d)
    promotionRepo := repository.NewPromotionRepository(d)
    assignmentQueueRepo := repository.NewAssignmentQueueRepository(d)
    therapistRepo := repository.NewTherapistRepository(d)
    offerRepo := repository.NewBookingOfferRepository(d)
    serviceRepo := repository.NewServiceRepository(d)
    addressRepo := repository.NewAddressRepository(d)
<<<<<<< HEAD
    extRepo := repository.NewExtensionRequestRepository(d)
    bookingService := service.NewBookingService(
		bookingRepo,
		promotionRepo,
		d,
		assignmentQueueRepo,
		therapistRepo,
		offerRepo,
		serviceRepo,
		addressRepo,
		repository.NewUserRepository(d), // userRepo
		nil, // MessageService
		nil, // NotificationService
		extRepo,
		nil, // WalletService
		nil, // RideService
	)

    // create service and address
    serviceID := createTestService(t, tx)
    addressID := createTestAddress(t, tx, int64(clientID), "", nil) // token/router nil is fine for direct DB helper
	
	// Convert serviceID string to int64 if needed
	var sID, aID int64
	fmt.Sscanf(serviceID, "%d", &sID)
	fmt.Sscanf(addressID, "%d", &aID)
=======
    bookingService := service.NewBookingService(bookingRepo, promotionRepo, d, assignmentQueueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, repository.NewUserRepository(d), nil, nil, repository.NewExtensionRequestRepository(d))
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

    req := &model.CreateBookingRequest{
        DurationMinutes: 60,
        ScheduledStart:  time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
        Notes:           "Admin created booking",
        GenderPref:      "any",
        PressurePref:    "medium",
        ServiceID:       &sID,
        AddressID:       &aID,
        PaymentMethod:   "cash",
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
        t.Fatalf("admin_created_booking event not recorded; events: %v", events)
    }
}
