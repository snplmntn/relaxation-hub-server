package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// Mocks for accept concurrency
type mockRepoAccept struct{
	mu sync.Mutex
	assigned bool
}

func (m *mockRepoAccept) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoAccept) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockRepoAccept) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, nil }
func (m *mockRepoAccept) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockRepoAccept) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoAccept) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockRepoAccept) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockRepoAccept) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockRepoAccept) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return &model.Booking{BookingID: bookingID}, nil }
func (m *mockRepoAccept) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockRepoAccept) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockRepoAccept) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockRepoAccept) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockRepoAccept) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }
func (m *mockRepoAccept) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockRepoAccept) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockRepoAccept) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockRepoAccept) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockRepoAccept) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockRepoAccept) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockRepoAccept) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockRepoAccept) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}

// We'll implement the guarded tx assign logic in the offer repo in this mock by
// failing the second assign via a shared flag.

type mockOfferRepoAccept struct{
	offers map[int64]*model.BookingOffer // key by therapistID
	mu sync.Mutex
}

func (m *mockOfferRepoAccept) Create(ctx context.Context, offer *model.BookingOffer) error { m.mu.Lock(); defer m.mu.Unlock(); m.offers[offer.TherapistID] = offer; return nil }
func (m *mockOfferRepoAccept) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoAccept) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o, ok := m.offers[therapistID]; ok {
		return []model.BookingOffer{*o}, nil
	}
	return []model.BookingOffer{}, nil
}
func (m *mockOfferRepoAccept) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	if o, ok := m.offers[therapistID]; ok { return o, nil }
	return nil, errors.New("no offer")
}
func (m *mockOfferRepoAccept) UpdateStatus(ctx context.Context, offerID int64, status string) error { return nil }
func (m *mockOfferRepoAccept) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error { return nil }
func (m *mockOfferRepoAccept) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoAccept) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoAccept) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }

// For concurrency we'll simulate AssignTherapistWithActorTx on the booking repo
// using a wrapper that performs an atomic check-and-set.

type mockBookingRepoAssign struct{
	mu sync.Mutex
	assigned bool
}
func (m *mockBookingRepoAssign) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAssign) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAssign) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAssign) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAssign) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockBookingRepoAssign) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockBookingRepoAssign) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockBookingRepoAssign) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.assigned {
		return repository.ErrAlreadyAssigned
	}
	m.assigned = true
	return nil
}
func (m *mockBookingRepoAssign) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return &model.Booking{BookingID: bookingID}, nil }
func (m *mockBookingRepoAssign) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockBookingRepoAssign) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error { return nil }
func (m *mockBookingRepoAssign) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockBookingRepoAssign) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockBookingRepoAssign) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }
func (m *mockBookingRepoAssign) GetBookingWithDetails(ctx context.Context, bookingID int64, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockBookingRepoAssign) GetBookingWithDetailsUnsafe(ctx context.Context, bookingID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: bookingID}}, nil
}
func (m *mockBookingRepoAssign) GetBookingByCodeWithDetails(ctx context.Context, referenceCode string, userID int64) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockBookingRepoAssign) GetBookingByCodeWithDetailsUnsafe(ctx context.Context, referenceCode string) (*repository.BookingDetailsResult, error) {
	return &repository.BookingDetailsResult{Booking: &model.Booking{BookingID: 1}}, nil
}
func (m *mockBookingRepoAssign) ListByClientWithDetails(ctx context.Context, clientID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAssign) ListByTherapistWithDetails(ctx context.Context, therapistID int64) ([]repository.BookingDetailsResult, error) {
	return []repository.BookingDetailsResult{}, nil
}
func (m *mockBookingRepoAssign) ListGlobalPending(ctx context.Context) ([]model.Booking, error) {
	return nil, nil
}
func (m *mockBookingRepoAssign) GetTherapistBookingCounts(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]int, error) {
	return map[int64]int{}, nil
}

// Minimal other mocks to satisfy NewBookingService
type noPromo struct{}
func (n *noPromo) Create(ctx context.Context, p *model.Promotion) error { return nil }
func (n *noPromo) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) { return nil, nil }
func (n *noPromo) GetByCode(ctx context.Context, code string) (*model.Promotion, error) { return nil, nil }
func (n *noPromo) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) { return true, nil }
func (n *noPromo) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) { return true, nil }

type noQueue struct{}
func (n *noQueue) Enqueue(ctx context.Context, bookingID int64) error { return nil }
func (n *noQueue) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) { return nil, nil }
func (n *noQueue) Remove(ctx context.Context, bookingID int64) error { return nil }
func (n *noQueue) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error { return nil }

type noTher struct{}
func (n *noTher) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) { return nil, nil }
func (n *noTher) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error { return nil }
func (n *noTher) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) { return nil, nil }
func (n *noTher) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error { return nil }
func (n *noTher) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) { return nil, nil }
func (n *noTher) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error { return nil }
func (n *noTher) AddService(ctx context.Context, ts *model.TherapistService) error { return nil }
func (n *noTher) RemoveService(ctx context.Context, therapistID, serviceID int64) error { return nil }
func (n *noTher) GetServices(ctx context.Context, therapistID int64) ([]int64, error) { return nil, nil }
func (n *noTher) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (n *noTher) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (n *noTher) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error { return nil }
func (n *noTher) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) { return map[int64][]string{}, nil }
func (n *noTher) CreateProfile(ctx context.Context, therapistID int64) error { return nil }
func (n *noTher) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) { return nil, nil }

func TestAcceptBookingOffer_Concurrent(t *testing.T) {
	ctx := context.Background()
	bookingRepo := &mockBookingRepoAssign{}
	offerRepo := &mockOfferRepoAccept{offers: map[int64]*model.BookingOffer{}}
	// create two offers for therapists 101 and 102
	offerRepo.Create(ctx, &model.BookingOffer{BookingID: 500, TherapistID: 101, Status: model.BookingOfferStatusPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)})
	offerRepo.Create(ctx, &model.BookingOffer{BookingID: 500, TherapistID: 102, Status: model.BookingOfferStatusPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)})

	svc := NewBookingService(bookingRepo, &noPromo{}, nil, &noQueue{}, &noTher{}, offerRepo, nil, nil, nil, nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan error, 2)

	go func() { defer wg.Done(); results <- svc.AcceptBookingOffer(ctx, 101, 500) }()
	go func() { defer wg.Done(); results <- svc.AcceptBookingOffer(ctx, 102, 500) }()
	wg.Wait()
	close(results)

	successes := 0
	errorsCount := 0
	for r := range results {
		if r == nil {
			successes++
		} else {
			errorsCount++
		}
	}
	if successes != 1 || errorsCount != 1 {
		t.Fatalf("expected 1 success and 1 error, got success=%d err=%d", successes, errorsCount)
	}
}

func TestAcceptBookingOffer_Expired(t *testing.T) {
	ctx := context.Background()
	bookingRepo := &mockBookingRepoAssign{}
	offerRepo := &mockOfferRepoAccept{offers: map[int64]*model.BookingOffer{}}
	offerRepo.Create(ctx, &model.BookingOffer{BookingID: 600, TherapistID: 111, Status: model.BookingOfferStatusPending, CreatedAt: time.Now().Add(-2 * time.Minute), ExpiresAt: time.Now().Add(-1 * time.Minute)})

	svc := NewBookingService(bookingRepo, &noPromo{}, nil, &noQueue{}, &noTher{}, offerRepo, nil, nil, nil, nil, nil)

	err := svc.AcceptBookingOffer(ctx, 111, 600)
	if err == nil || err.Error() != "offer expired" {
		t.Fatalf("expected offer expired error, got: %v", err)
	}
}

// Payment update ordering test: ensure PaymentService.UpdateStatus calls into repo and returns payment
func TestPaymentService_UpdateStatus(t *testing.T) {
	tctx := context.Background()
	mock := &mockPaymentRepo{}
	svc := NewPaymentService(mock)

	p, err := svc.UpdateStatus(tctx, 700, &model.UpdatePaymentStatusRequest{Status: "paid", TransactionID: ptrString("tx123"), WebhookID: ptrString("wh1")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil || p.Status != "paid" {
		t.Fatalf("expected payment status 'paid', got: %+v", p)
	}
}

func ptrString(s string) *string { return &s }

// mockPaymentRepo implements minimal PaymentRepository
type mockPaymentRepo struct{}
func (m *mockPaymentRepo) Create(ctx context.Context, p *model.Payment) error { return nil }
func (m *mockPaymentRepo) GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error) { return &model.Payment{BookingID: bookingID, Status: "paid"}, nil }
func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error { return nil }

