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
	profiles      []model.TherapistProfile
	findErr       error
	genderFilter  string
	pressureFilter string
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

type fakeBookingRepoForMatching struct {
	repository.BookingRepository
	struggleFlags  map[int64]bool
	bookingCounts  map[int64]int
	struggleErr    error
	countsErr      error
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
