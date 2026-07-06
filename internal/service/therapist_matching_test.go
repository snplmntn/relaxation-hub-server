package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// --- Mocks ---

type fakeTherapistRepo struct {
	repository.TherapistRepository
	profiles        []model.TherapistProfile
	findErr         error
	genderFilter    string
	pressureFilter  string
	slotWindowStart time.Time
	slotWindowEnd   time.Time
	slotQuantity    int
	slotStarts      []time.Time
	availability    map[time.Time]bool
}

func (f *fakeTherapistRepo) FindAvailableByService(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	f.genderFilter = genderPreference
	f.pressureFilter = pressurePreference
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.profiles, nil
}

func (f *fakeTherapistRepo) FindNearbyByService(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	latitude float64,
	longitude float64,
	radiusKm float64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	f.genderFilter = genderPreference
	f.pressureFilter = pressurePreference
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.profiles, nil
}

func (f *fakeTherapistRepo) HasAvailableTherapists(
	ctx context.Context,
	windowStart time.Time,
	windowEnd time.Time,
	quantity int,
) (bool, error) {
	f.slotWindowStart = windowStart
	f.slotWindowEnd = windowEnd
	f.slotQuantity = quantity
	slotStart := windowStart.Add(availabilityTravelBufferMinutes * time.Minute)
	f.slotStarts = append(f.slotStarts, slotStart)
	if f.availability != nil {
		return f.availability[slotStart], nil
	}
	return true, nil
}

type fakeBookingRepoForMatching struct {
	repository.BookingRepository
	struggleFlags map[int64]bool
	bookingCounts map[int64]int
	struggleErr   error
	countsErr     error
}

func (f *fakeBookingRepoForMatching) GetRecentTherapistStruggleFlags(
	ctx context.Context,
	therapistIDs []int64,
	since time.Time,
) (map[int64]bool, error) {
	if f.struggleErr != nil {
		return nil, f.struggleErr
	}
	if f.struggleFlags == nil {
		return map[int64]bool{}, nil
	}
	return f.struggleFlags, nil
}

func (f *fakeBookingRepoForMatching) GetTherapistBookingCounts(
	ctx context.Context,
	therapistIDs []int64,
	since time.Time,
) (map[int64]int, error) {
	if f.countsErr != nil {
		return nil, f.countsErr
	}
	if f.bookingCounts == nil {
		return map[int64]int{}, nil
	}
	return f.bookingCounts, nil
}

// --- Tests ---

func TestTherapistMatchingService_FindAvailable_InvalidServiceID(t *testing.T) {
	svc := NewTherapistMatchingService(&fakeTherapistRepo{}, &fakeBookingRepoForMatching{})

	_, err := svc.FindAvailableTherapistsForService(context.Background(), 1, 0, "", "")
	if err == nil {
		t.Fatal("expected error for invalid service_id, got nil")
	}
	if err.Error() != "invalid service_id: must be positive" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTherapistMatchingService_FindAvailable_InvalidGender(t *testing.T) {
	svc := NewTherapistMatchingService(&fakeTherapistRepo{}, &fakeBookingRepoForMatching{})

	_, err := svc.FindAvailableTherapistsForService(context.Background(), 1, 1, "invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid gender, got nil")
	}
}

func TestTherapistMatchingService_FindAvailable_Success(t *testing.T) {
	therapists := []model.TherapistProfile{
		{TherapistID: 1, AvgRating: 5.0},
		{TherapistID: 2, AvgRating: 4.5},
	}
	therapistRepo := &fakeTherapistRepo{profiles: therapists}
	bookingRepo := &fakeBookingRepoForMatching{
		struggleFlags: map[int64]bool{},
		bookingCounts: map[int64]int{1: 10, 2: 10},
	}
	svc := NewTherapistMatchingService(therapistRepo, bookingRepo)

	result, err := svc.FindAvailableTherapistsForService(context.Background(), 100, 1, "female", "soft")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 therapists, got %d", len(result))
	}
	if therapistRepo.genderFilter != "female" {
		t.Errorf("expected gender filter 'female', got '%s'", therapistRepo.genderFilter)
	}
}

func TestTherapistMatchingService_FindAvailable_BoostsStrugglingTherapists(t *testing.T) {
	therapists := []model.TherapistProfile{
		{TherapistID: 1, AvgRating: 5.0},
		{TherapistID: 2, AvgRating: 4.0}, // this one has low volume
	}
	therapistRepo := &fakeTherapistRepo{profiles: therapists}
	// Low volume boosting: therapist 2 has only 1 booking (50% of average 10.5/2=5.25, so 1 < 2.6)
	// Average = (10 + 1) / 2 = 5.5, 50% of 5.5 = 2.75. Therapist 2 with 1 booking qualifies.
	bookingRepo := &fakeBookingRepoForMatching{
		struggleFlags: map[int64]bool{},
		bookingCounts: map[int64]int{1: 10, 2: 1}, // therapist 2 has low volume
	}
	svc := NewTherapistMatchingService(therapistRepo, bookingRepo)

	result, err := svc.FindAvailableTherapistsForService(context.Background(), 100, 1, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Low-volume therapist(id=2) should be first
	if result[0].TherapistID != 2 {
		t.Errorf("expected low-volume therapist (id=2) to be boosted first, got id=%d", result[0].TherapistID)
	}
}

func TestTherapistMatchingService_FindAvailable_EmptyResult(t *testing.T) {
	therapistRepo := &fakeTherapistRepo{profiles: []model.TherapistProfile{}}
	bookingRepo := &fakeBookingRepoForMatching{}
	svc := NewTherapistMatchingService(therapistRepo, bookingRepo)

	result, err := svc.FindAvailableTherapistsForService(context.Background(), 100, 1, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}

func TestTherapistMatchingService_FindAvailable_RepoError(t *testing.T) {
	therapistRepo := &fakeTherapistRepo{findErr: errors.New("db error")}
	bookingRepo := &fakeBookingRepoForMatching{}
	svc := NewTherapistMatchingService(therapistRepo, bookingRepo)

	_, err := svc.FindAvailableTherapistsForService(context.Background(), 100, 1, "", "")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

func TestTherapistMatchingService_FindNearby_InvalidLocation(t *testing.T) {
	svc := NewTherapistMatchingService(&fakeTherapistRepo{}, &fakeBookingRepoForMatching{})

	// Location outside Philippines
	_, err := svc.FindNearbyAvailableTherapists(context.Background(), 1, 1, 50.0, 130.0, 10.0, "", "")
	if err == nil {
		t.Fatal("expected location error, got nil")
	}
}

func TestTherapistMatchingService_FindNearby_InvalidRadius(t *testing.T) {
	svc := NewTherapistMatchingService(&fakeTherapistRepo{}, &fakeBookingRepoForMatching{})

	// Invalid radius
	_, err := svc.FindNearbyAvailableTherapists(context.Background(), 1, 1, 14.0, 121.0, 0, "", "")
	if err == nil {
		t.Fatal("expected radius error, got nil")
	}
	_, err = svc.FindNearbyAvailableTherapists(context.Background(), 1, 1, 14.0, 121.0, 200, "", "")
	if err == nil {
		t.Fatal("expected radius error for >100km, got nil")
	}
}

func TestTherapistMatchingService_FindNearby_Success(t *testing.T) {
	therapists := []model.TherapistProfile{
		{TherapistID: 1, AvgRating: 5.0},
	}
	therapistRepo := &fakeTherapistRepo{profiles: therapists}
	bookingRepo := &fakeBookingRepoForMatching{}
	svc := NewTherapistMatchingService(therapistRepo, bookingRepo)

	result, err := svc.FindNearbyAvailableTherapists(context.Background(), 1, 1, 14.5, 121.0, 10.0, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 therapist, got %d", len(result))
	}
}

func TestTherapistMatchingService_IsSlotAvailable_UsesDurationAndQuantity(t *testing.T) {
	therapistRepo := &fakeTherapistRepo{}
	svc := NewTherapistMatchingService(therapistRepo, &fakeBookingRepoForMatching{})
	start := time.Date(2026, time.July, 6, 23, 0, 0, 0, time.FixedZone("PHT", 8*60*60))

	available, err := svc.IsSlotAvailable(context.Background(), start, 120, 2)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Fatal("expected available")
	}
	if !therapistRepo.slotWindowStart.Equal(start.Add(-30 * time.Minute)) {
		t.Fatalf("unexpected window start: %v", therapistRepo.slotWindowStart)
	}
	if !therapistRepo.slotWindowEnd.Equal(start.Add(150 * time.Minute)) {
		t.Fatalf("unexpected window end: %v", therapistRepo.slotWindowEnd)
	}
	if therapistRepo.slotQuantity != 2 {
		t.Fatalf("expected quantity 2, got %d", therapistRepo.slotQuantity)
	}
}

func TestTherapistMatchingService_FindAlternativeSlots_UsesThirtyMinuteOpenSlots(t *testing.T) {
	start := time.Date(2026, time.July, 6, 19, 0, 0, 0, time.FixedZone("PHT", 8*60*60))
	therapistRepo := &fakeTherapistRepo{
		availability: map[time.Time]bool{
			start.Add(30 * time.Minute):  false,
			start.Add(60 * time.Minute):  true,
			start.Add(90 * time.Minute):  false,
			start.Add(120 * time.Minute): true,
			start.Add(150 * time.Minute): true,
		},
	}
	svc := NewTherapistMatchingService(therapistRepo, &fakeBookingRepoForMatching{})

	slots, err := svc.FindAlternativeSlots(context.Background(), start, 120, 2, 2)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if !slots[0].Start.Equal(start.Add(60 * time.Minute)) {
		t.Fatalf("unexpected first alternative: %v", slots[0].Start)
	}
	if !slots[1].Start.Equal(start.Add(120 * time.Minute)) {
		t.Fatalf("unexpected second alternative: %v", slots[1].Start)
	}
	if therapistRepo.slotQuantity != 2 {
		t.Fatalf("expected quantity 2, got %d", therapistRepo.slotQuantity)
	}
}

func TestTherapistMatchingService_FindAlternativeSlots_StaysOnRequestedDate(t *testing.T) {
	start := time.Date(2026, time.July, 6, 23, 30, 0, 0, time.FixedZone("PHT", 8*60*60))
	therapistRepo := &fakeTherapistRepo{}
	svc := NewTherapistMatchingService(therapistRepo, &fakeBookingRepoForMatching{})

	slots, err := svc.FindAlternativeSlots(context.Background(), start, 60, 1, 3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slots) != 0 {
		t.Fatalf("expected no next-day alternatives, got %d", len(slots))
	}
	if len(therapistRepo.slotStarts) != 0 {
		t.Fatalf("expected no availability scans after date boundary, got %d", len(therapistRepo.slotStarts))
	}
}
