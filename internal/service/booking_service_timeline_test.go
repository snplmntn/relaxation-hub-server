package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// mockBookingRepoTimeline satisfies repository.BookingRepository for tests
type mockBookingRepoTimeline struct {
    booking *model.Booking
    events  []model.BookingEvent
    eventsErr error
}

func (m *mockBookingRepoTimeline) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoTimeline) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoTimeline) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
    return m.booking, nil
}
func (m *mockBookingRepoTimeline) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoTimeline) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoTimeline) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoTimeline) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoTimeline) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoTimeline) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return m.booking, nil }
func (m *mockBookingRepoTimeline) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoTimeline) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }
func (m *mockBookingRepoTimeline) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) {
    if m.eventsErr != nil { return nil, m.eventsErr }
    return m.events, nil
}

func (m *mockBookingRepoTimeline) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
    return nil
}

func (m *mockBookingRepoTimeline) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoTimeline) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
    if m.booking == nil {
        return nil, pgx.ErrNoRows
    }
    return &repository.BookingDetailsResult{
        Booking: m.booking,
        ClientName: "Test Client",
        ClientPhone: "1234567890",
        ClientPhoto: "",
    }, nil
}
func (m *mockBookingRepoTimeline) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
    return m.GetBookingWithDetails(ctx, bookingID, 0)
}
func (m *mockBookingRepoTimeline) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
    if m.booking == nil {
        return nil, pgx.ErrNoRows
    }
    return &repository.BookingDetailsResult{
        Booking: m.booking,
        ClientName: "Test Client",
        ClientPhone: "1234567890",
    }, nil
}
func (m *mockBookingRepoTimeline) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
    return m.GetBookingByCodeWithDetails(ctx, referenceCode, 0)
}
func (m *mockBookingRepoTimeline) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
    return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoTimeline) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
    return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoTimeline) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
    return nil, nil
}
func (m *mockBookingRepoTimeline) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
    return map[int64]int{}, nil
}

func TestGetBookingWithTimeline_Success(t *testing.T) {
    now := time.Now()
    b := &model.Booking{BookingID: 1, ClientID: 10, Status: "pending", CreatedAt: now, UpdatedAt: now}
    events := []model.BookingEvent{{EventID: 1, BookingID: 1, EventType: "created", CreatedAt: now}}

    mock := &mockBookingRepoTimeline{booking: b, events: events}
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    gotB, gotEvents, _, _, _, _, _, _, _, _, _, _, _, _, err := svc.GetBookingWithTimeline(context.Background(), 1, 10, "client")
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if !reflect.DeepEqual(gotB, b) { t.Fatalf("expected booking %+v, got %+v", b, gotB) }
    if !reflect.DeepEqual(gotEvents, events) { t.Fatalf("expected events %+v, got %+v", events, gotEvents) }
}

func TestGetBookingWithTimeline_EventsError(t *testing.T) {
    now := time.Now()
    b := &model.Booking{BookingID: 2, ClientID: 20, Status: "pending", CreatedAt: now, UpdatedAt: now}

    mock := &mockBookingRepoTimeline{booking: b, eventsErr: errors.New("db failure")}
    svc := NewBookingService(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

    gotB, gotEvents, _, _, _, _, _, _, _, _, _, _, _, _, err := svc.GetBookingWithTimeline(context.Background(), 2, 20, "client")
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if gotB.BookingID != b.BookingID { t.Fatalf("expected booking id %d, got %d", b.BookingID, gotB.BookingID) }
    if len(gotEvents) != 0 { t.Fatalf("expected empty events on repo error, got %+v", gotEvents) }
}
