package service

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type TherapistMatchingService interface {
	FindAvailableTherapistsForService(
		ctx context.Context,
		serviceID int64,
		genderPreference string,
		pressurePreference string,
	) ([]model.TherapistProfile, error)

	FindNearbyAvailableTherapists(
		ctx context.Context,
		serviceID int64,
		latitude float64,
		longitude float64,
		radiusKm float64,
		genderPreference string,
		pressurePreference string,
	) ([]model.TherapistProfile, error)
}

type therapistMatchingService struct {
	therapistRepo repository.TherapistRepository
	bookingRepo   repository.BookingRepository
}

// NewTherapistMatchingService creates a new therapist matching service
func NewTherapistMatchingService(
	therapistRepo repository.TherapistRepository,
	bookingRepo repository.BookingRepository,
) TherapistMatchingService {
	return &therapistMatchingService{
		therapistRepo: therapistRepo,
		bookingRepo:   bookingRepo,
	}
}

// FindAvailableTherapistsForService finds all available therapists offering a specific service
// Filters by gender preference and returns them ordered by rating
func (s *therapistMatchingService) FindAvailableTherapistsForService(
	ctx context.Context,
	serviceID int64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	if serviceID <= 0 {
		return nil, fmt.Errorf("invalid service_id: must be positive")
	}

	// Validate gender preference (allow 'any', 'male', 'female', or empty for all)
	validGenders := map[string]bool{
		"":       true,
		"any":    true,
		"male":   true,
		"female": true,
	}
	if !validGenders[genderPreference] {
		return nil, fmt.Errorf("invalid gender preference: must be 'male', 'female', or 'any'")
	}

	therapists, err := s.therapistRepo.FindAvailableByService(ctx, serviceID, genderPreference, pressurePreference)
	if err != nil {
		return nil, fmt.Errorf("failed to find available therapists: %w", err)
	}

	if len(therapists) == 0 {
		return []model.TherapistProfile{}, nil
	}

	// Repository now only returns therapists with `accept_assignments = true`.
	if len(therapists) == 0 {
		return []model.TherapistProfile{}, nil
	}

	// Boost therapists who had recent struggles (cancellations/no-shows) in the
	// last 24 hours so they get prioritized when they are available.
	ids := make([]int64, 0, len(therapists))
	for _, t := range therapists {
		ids = append(ids, t.TherapistID)
	}

	since := time.Now().Add(-24 * time.Hour)
	struggleMap, err := s.bookingRepo.GetRecentTherapistStruggleFlags(ctx, ids, since)
	if err != nil {
		// non-fatal: return original list if we cannot compute struggles
		return therapists, nil
	}

	// Partition into struggling and others (preserve original ordering within each group)
	struggling := make([]model.TherapistProfile, 0)
	others := make([]model.TherapistProfile, 0)
	for _, t := range therapists {
		if struggleMap[t.TherapistID] {
			struggling = append(struggling, t)
		} else {
			others = append(others, t)
		}
	}

	// Return struggling therapists first so they are attempted earlier by the worker
	result := make([]model.TherapistProfile, 0, len(therapists))
	result = append(result, struggling...)
	result = append(result, others...)

	therapists = result

	return therapists, nil
}

// FindNearbyAvailableTherapists finds available therapists within a radius using geospatial queries
// Pre-computes and returns closest, highest-rated therapists first
func (s *therapistMatchingService) FindNearbyAvailableTherapists(
	ctx context.Context,
	serviceID int64,
	latitude float64,
	longitude float64,
	radiusKm float64,
	genderPreference string,
	pressurePreference string,
) ([]model.TherapistProfile, error) {
	if serviceID <= 0 {
		return nil, fmt.Errorf("invalid service_id: must be positive")
	}

	if latitude < 5 || latitude > 20 || longitude < 116 || longitude > 127 {
		return nil, fmt.Errorf("location out of service area (Philippines only)")
	}

	if radiusKm <= 0 || radiusKm > 100 {
		return nil, fmt.Errorf("invalid radius: must be between 0 and 100 km")
	}

	validGenders := map[string]bool{
		"":       true,
		"any":    true,
		"male":   true,
		"female": true,
	}
	if !validGenders[genderPreference] {
		return nil, fmt.Errorf("invalid gender preference")
	}

	therapists, err := s.therapistRepo.FindNearbyByService(
		ctx,
		serviceID,
		latitude,
		longitude,
		radiusKm,
		genderPreference,
		pressurePreference,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby therapists: %w", err)
	}

	return therapists, nil
}

// canProvidePressure checks if a therapist can provide the requested pressure level
func (s *therapistMatchingService) canProvidePressure(
	offeredPressures []string,
	requestedPressure string,
) bool {
	if len(offeredPressures) == 0 {
		return false
	}

	for _, p := range offeredPressures {
		if p == requestedPressure || p == "all" {
			return true
		}
	}
	return false
}
