package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// Mocks for testing offer creation
type mockRepoForOffers struct {
	createdBookingID int64
	insertedEvents   []struct{
		bookingID int64
		eventType string
		actorID  *int64
		metadata map[string]any
	}
}

func (m *mockRepoForOffers) Create(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoForOffers) CreateTx(ctx context.Context, tx pgx.Tx, booking *model.Booking) error { booking.BookingID = 999; m.createdBookingID = 999; return nil }
func (m *mockRepoForOffers) GetByID(ctx context.Context, bookingID, userID int64) (*model.Booking, error) { return nil, nil }
func (m *mockRepoForOffers) ListByClient(ctx context.Context, clientID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockRepoForOffers) Update(ctx context.Context, booking *model.Booking) error { return nil }
func (m *mockRepoForOffers) AssignTherapist(ctx context.Context, bookingID, therapistID int64) error { return nil }
func (m *mockRepoForOffers) AssignTherapistWithActor(ctx context.Context, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockRepoForOffers) AssignTherapistWithActorTx(ctx context.Context, tx pgx.Tx, bookingID, therapistID, actorID int64) error { return nil }
func (m *mockRepoForOffers) GetByBookingID(ctx context.Context, bookingID int64) (*model.Booking, error) { return nil, nil }
func (m *mockRepoForOffers) ListEvents(ctx context.Context, bookingID int64) ([]model.BookingEvent, error) { return nil, nil }
func (m *mockRepoForOffers) InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error {
	m.insertedEvents = append(m.insertedEvents, struct{
		bookingID int64
		eventType string
		actorID  *int64
		metadata map[string]any
	}{bookingID, eventType, actorID, metadata})
	return nil
}
func (m *mockRepoForOffers) UpdateStatus(ctx context.Context, bookingID, actorID int64, status string, cancelledBy *string, cancellationReason *string) error { return nil }
func (m *mockRepoForOffers) ListByTherapist(ctx context.Context, therapistID int64) ([]model.Booking, error) { return nil, nil }
func (m *mockRepoForOffers) GetRecentTherapistStruggleFlags(ctx context.Context, therapistIDs []int64, since time.Time) (map[int64]bool, error) { return map[int64]bool{}, nil }

// mockOfferRepo captures created offers
type mockOfferRepoForTest struct{
	created []*model.BookingOffer
}
func (m *mockOfferRepoForTest) Create(ctx context.Context, offer *model.BookingOffer) error { offer.OfferID = int64(len(m.created)+1); m.created = append(m.created, offer); return nil }
func (m *mockOfferRepoForTest) GetActiveOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoForTest) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) { return []model.BookingOffer{}, nil }
func (m *mockOfferRepoForTest) GetByTherapistAndBooking(ctx context.Context, therapistID, bookingID int64) (*model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoForTest) UpdateStatus(ctx context.Context, offerID int64, status string) error { return nil }
func (m *mockOfferRepoForTest) UpdateStatusTx(ctx context.Context, tx pgx.Tx, offerID int64, status string) error { return nil }
func (m *mockOfferRepoForTest) ExpireOffers(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoForTest) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }
func (m *mockOfferRepoForTest) GetOffersByBookingID(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) { return nil, nil }

// mockTherapistRepo returns candidates
type mockTherapistRepoForTest struct{}
func (m *mockTherapistRepoForTest) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoForTest) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error { return nil }
func (m *mockTherapistRepoForTest) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoForTest) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error { return nil }
func (m *mockTherapistRepoForTest) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) { return nil, nil }
func (m *mockTherapistRepoForTest) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error { return nil }
func (m *mockTherapistRepoForTest) AddService(ctx context.Context, ts *model.TherapistService) error { return nil }
func (m *mockTherapistRepoForTest) RemoveService(ctx context.Context, therapistID, serviceID int64) error { return nil }
func (m *mockTherapistRepoForTest) GetServices(ctx context.Context, therapistID int64) ([]int64, error) { return nil, nil }
func (m *mockTherapistRepoForTest) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error { return nil }
func (m *mockTherapistRepoForTest) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) { return map[int64][]string{}, nil }
func (m *mockTherapistRepoForTest) CreateProfile(ctx context.Context, therapistID int64) error { return nil }
func (m *mockTherapistRepoForTest) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return []model.TherapistProfile{{TherapistID: 101}, {TherapistID: 102}, {TherapistID: 103}}, nil
}
func (m *mockTherapistRepoForTest) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) { return nil, nil }
func (m *mockTherapistRepoForTest) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) { return nil, nil }

// minimal mocks for other dependencies
type nilPromoRepo struct{}
func (n *nilPromoRepo) Create(ctx context.Context, p *model.Promotion) error { return nil }
func (n *nilPromoRepo) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) { return nil, nil }
func (n *nilPromoRepo) GetByCode(ctx context.Context, code string) (*model.Promotion, error) { return nil, nil }
func (n *nilPromoRepo) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) { return true, nil }
func (n *nilPromoRepo) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) { return true, nil }

type nilQueueRepo struct{}
func (n *nilQueueRepo) Enqueue(ctx context.Context, bookingID int64) error { return nil }
func (n *nilQueueRepo) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) { return nil, nil }
func (n *nilQueueRepo) Remove(ctx context.Context, bookingID int64) error { return nil }
func (n *nilQueueRepo) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error { return nil }

func TestCreate_CreatesOffersAndEvents(t *testing.T) {
	ctx := context.Background()
	mockBooking := &mockRepoForOffers{}
	mockOffer := &mockOfferRepoForTest{}
	mockTher := &mockTherapistRepoForTest{}
	promo := &nilPromoRepo{}
	queue := &nilQueueRepo{}

	svc := NewBookingService(mockBooking, promo, nil, queue, mockTher, mockOffer, nil, nil, nil, nil)

	req := &model.CreateBookingRequest{ServiceID: ptrInt64(10), DurationMinutes: 60}
	b, err := svc.Create(ctx, 11, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatalf("expected booking returned")
	}
	// verify offers created
	if len(mockOffer.created) == 0 {
		t.Fatalf("expected offers to be created, got 0")
	}
	if mockOffer.created[0].BookingID != mockBooking.createdBookingID {
		t.Fatalf("offer BookingID mismatch: got %d want %d", mockOffer.created[0].BookingID, mockBooking.createdBookingID)
	}
	// verify events inserted for offers: look for offered_to_therapist
	found := false
	for _, ev := range mockBooking.insertedEvents {
		if ev.eventType == "offered_to_therapist" {
			found = true
			if ev.metadata == nil {
				t.Fatalf("expected metadata on offered_to_therapist event")
			}
			// metadata should contain offer_id and target_therapist_id
			if _, ok := ev.metadata["offer_id"]; !ok {
				t.Fatalf("metadata missing offer_id")
			}
			if _, ok := ev.metadata["target_therapist_id"]; !ok {
				t.Fatalf("metadata missing target_therapist_id")
			}
		}
	}
	if !found {
		t.Fatalf("offered_to_therapist event not inserted")
	}
}

func ptrInt64(v int64) *int64 { return &v }
