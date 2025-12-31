package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockBookingRepoAdmin is a minimal BookingRepository for admin-create tests
type mockBookingRepoAdmin struct {
	createdBooking *model.Booking
	assignErr      error
}

func (m *mockBookingRepoAdmin) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAdmin) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error {
	// simulate DB assigning an ID
	booking.BookingID = 555
	m.createdBooking = booking
	return nil
}
func (m *mockBookingRepoAdmin) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, pgx.ErrNoRows }
func (m *mockBookingRepoAdmin) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAdmin) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAdmin) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoAdmin) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoAdmin) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return m.assignErr }
func (m *mockBookingRepoAdmin) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) {
	if m.createdBooking == nil { return nil, pgx.ErrNoRows }
	return m.createdBooking, nil
}
func (m *mockBookingRepoAdmin) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return []model.BookingEvent{}, nil }
func (m *mockBookingRepoAdmin) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockBookingRepoAdmin) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoAdmin) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAdmin) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }
func (m *mockBookingRepoAdmin) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: m.createdBooking}, nil
}
func (m *mockBookingRepoAdmin) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAdmin) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAdmin) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAdmin) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}


// mockTherapistRepoAdmin controls GetProfile behavior
type mockTherapistRepoAdmin struct {
	profile *model.TherapistProfile
	err     error
}
func (m *mockTherapistRepoAdmin) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	if m.err != nil { return nil, m.err }
	return m.profile, nil
}
func (m *mockTherapistRepoAdmin) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error { return nil }
func (m *mockTherapistRepoAdmin) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoAdmin) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error { return nil }
func (m *mockTherapistRepoAdmin) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) { return nil, nil }
func (m *mockTherapistRepoAdmin) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error { return nil }
func (m *mockTherapistRepoAdmin) AddService(ctx context.Context, ts *model.TherapistService) error { return nil }
func (m *mockTherapistRepoAdmin) RemoveService(ctx context.Context, therapistID, serviceID int64) error { return nil }
func (m *mockTherapistRepoAdmin) GetServices(ctx context.Context, therapistID int64) ([]int64, error) { return nil, nil }
func (m *mockTherapistRepoAdmin) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error { return nil }
func (m *mockTherapistRepoAdmin) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) { return map[int64][]string{}, nil }
func (m *mockTherapistRepoAdmin) CreateProfile(ctx context.Context, therapistID int64) error { return nil }
func (m *mockTherapistRepoAdmin) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoAdmin) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoAdmin) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) { return nil, nil }

func TestAdminCreate_Assignment_TherapistNotFound(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{}
	tr := &mockTherapistRepoAdmin{err: pgx.ErrNoRows}
	svc := NewBookingService(br, nil, nil, nil, tr, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when therapist not found")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "invalid_therapist" {
		t.Fatalf("expected code invalid_therapist, got %s", ve.Code)
	}
}

func TestAdminCreate_Assignment_TherapistNotAccepting(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{}
	tr := &mockTherapistRepoAdmin{profile: &model.TherapistProfile{TherapistID: 9, AcceptAssignments: false}}
	svc := NewBookingService(br, nil, nil, nil, tr, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when therapist not accepting")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "therapist_not_accepting" {
		t.Fatalf("expected code therapist_not_accepting, got %s", ve.Code)
	}
}

func TestAdminCreate_Assignment_RaceConditionAssignFails(t *testing.T) {
	ctx := context.Background()
	br := &mockBookingRepoAdmin{assignErr: pgx.ErrNoRows}
	tr := &mockTherapistRepoAdmin{profile: &model.TherapistProfile{TherapistID: 9, AcceptAssignments: true}}
	svc := NewBookingService(br, nil, nil, nil, tr, nil, nil, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{DurationMinutes: 60, TherapistID: func() *int64 { v := int64(9); return &v }()}
	_, err := svc.CreateForAdmin(ctx, 1, 2, req)
	if err == nil {
		t.Fatalf("expected error when assign fails due to gating race")
	}
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	} else if ve.Code != "cannot_assign" {
		t.Fatalf("expected code cannot_assign, got %s", ve.Code)
	}
}
