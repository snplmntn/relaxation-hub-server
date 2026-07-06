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
		clientID int64,
		serviceID int64,
		genderPreference string,
		pressurePreference string,
	) ([]model.TherapistProfile, error)

	FindNearbyAvailableTherapists(
		ctx context.Context,
		clientID int64,
		serviceID int64,
		latitude float64,
		longitude float64,
		radiusKm float64,
		genderPreference string,
		pressurePreference string,
	) ([]model.TherapistProfile, error)

	FindAvailableTherapistsForServiceWithTime(
		ctx context.Context,
		clientID int64,
		serviceID int64,
		genderPreference string,
		pressurePreference string,
		scheduledStart time.Time,
		durationMinutes int,
		lat *float64,
		lng *float64,
	) ([]model.TherapistProfile, error)

	// IsSlotAvailable reports whether enough therapists are free for a slot
	// starting at scheduledStart. Service-agnostic — used by the public
	// availability check.
	IsSlotAvailable(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity int) (bool, error)

	// FindAlternativeSlots returns nearby future slots on the same local date
	// that pass the same availability check as IsSlotAvailable.
	FindAlternativeSlots(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity, limit int) ([]AvailabilitySlot, error)
}

// Calibration knobs for the service-agnostic availability check. The agent
// sends no service (so no real duration) and no coordinates (so no distance-
// based buffer), so we assume a conservative slot and pad both sides.
const (
	// ponytail: fixed travel pad in place of the distance-based buffer used in
	// FindAvailableByServiceWithTime; widen if home-service travel runs longer.
	availabilityTravelBufferMinutes = 30
	availabilityAlternativeStep     = 30 * time.Minute
	availabilityAlternativeHorizon  = 8 * time.Hour
)

// AvailabilitySlot is an anonymous, customer-safe slot candidate. It carries no
// therapist identity; handlers format it for the chat agent response.
type AvailabilitySlot struct {
	Start time.Time
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

// IsSlotAvailable reports whether enough therapists could take a slot starting
// at scheduledStart, padding the conflict window on both sides by the travel buffer.
func (s *therapistMatchingService) IsSlotAvailable(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity int) (bool, error) {
	durationMinutes, quantity = normalizeAvailabilityInputs(durationMinutes, quantity)
	windowStart, windowEnd := availabilityWindow(scheduledStart, durationMinutes)
	return s.therapistRepo.HasAvailableTherapists(ctx, windowStart, windowEnd, quantity)
}

// FindAlternativeSlots scans forward on a 30-minute grid and returns the first
// nearby slots that pass the same buffered therapist-availability check.
func (s *therapistMatchingService) FindAlternativeSlots(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity, limit int) ([]AvailabilitySlot, error) {
	if limit <= 0 {
		return []AvailabilitySlot{}, nil
	}
	durationMinutes, quantity = normalizeAvailabilityInputs(durationMinutes, quantity)
	localDate := scheduledStart.Format("2006-01-02")
	deadline := scheduledStart.Add(availabilityAlternativeHorizon)
	slots := make([]AvailabilitySlot, 0, limit)

	for candidate := scheduledStart.Add(availabilityAlternativeStep); !candidate.After(deadline); candidate = candidate.Add(availabilityAlternativeStep) {
		if candidate.Format("2006-01-02") != localDate {
			break
		}
		available, err := s.IsSlotAvailable(ctx, candidate, durationMinutes, quantity)
		if err != nil {
			return nil, err
		}
		if !available {
			continue
		}
		slots = append(slots, AvailabilitySlot{Start: candidate})
		if len(slots) >= limit {
			break
		}
	}

	return slots, nil
}

func normalizeAvailabilityInputs(durationMinutes, quantity int) (int, int) {
	if durationMinutes <= 0 {
		durationMinutes = 60
	}
	if quantity <= 0 {
		quantity = 1
	}
	return durationMinutes, quantity
}

func availabilityWindow(scheduledStart time.Time, durationMinutes int) (time.Time, time.Time) {
	buffer := availabilityTravelBufferMinutes * time.Minute
	return scheduledStart.Add(-buffer), scheduledStart.Add(time.Duration(durationMinutes)*time.Minute + buffer)
}

// FindAvailableTherapistsForService finds all available therapists offering a specific service
// Filters by gender preference and returns them ordered by rating
func (s *therapistMatchingService) FindAvailableTherapistsForService(
	ctx context.Context,
	clientID int64,
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

	therapists, err := s.therapistRepo.FindAvailableByService(ctx, clientID, serviceID, genderPreference, pressurePreference)
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

	// Apply fairness sorting using shared helper
	return s.applyFairnessSort(ctx, therapists), nil
}

// applyFairnessSort sorts therapists by booking volume (low-volume first) to distribute work fairly.
// This is extracted as a helper to avoid duplication between matching methods.
func (s *therapistMatchingService) applyFairnessSort(ctx context.Context, therapists []model.TherapistProfile) []model.TherapistProfile {
	if len(therapists) == 0 {
		return therapists
	}

	// Extract therapist IDs for batch query
	ids := make([]int64, 0, len(therapists))
	for _, t := range therapists {
		ids = append(ids, t.TherapistID)
	}

	// Get booking counts to identify therapists with significantly fewer bookings
	// Use a 24-hour window to assess recent volume
	countsSince := time.Now().Add(-24 * time.Hour)
	bookingCounts, err := s.bookingRepo.GetTherapistBookingCounts(ctx, ids, countsSince)
	if err != nil {
		bookingCounts = map[int64]int{} // non-fatal: continue without counts
	}

	// Calculate average bookings among candidates (treat missing as 0)
	totalBookings := 0
	for _, tid := range ids {
		totalBookings += bookingCounts[tid]
	}
	avgBookings := 0.0
	if len(ids) > 0 {
		avgBookings = float64(totalBookings) / float64(len(ids))
	}

	// Therapists with 50% or less of the average are considered "low volume" and get boosted
	lowVolumeThreshold := avgBookings * 0.5

	// Partition into low-volume (struggling) and others
	struggling := make([]model.TherapistProfile, 0)
	others := make([]model.TherapistProfile, 0)
	for _, t := range therapists {
		isLowVolume := float64(bookingCounts[t.TherapistID]) <= lowVolumeThreshold
		if isLowVolume {
			struggling = append(struggling, t)
		} else {
			others = append(others, t)
		}
	}

	// Return struggling therapists first so they are attempted earlier by the worker
	result := make([]model.TherapistProfile, 0, len(therapists))
	result = append(result, struggling...)
	result = append(result, others...)

	return result
}

// FindAvailableTherapistsForServiceWithTime finds all available therapists offering a specific service
// checking for time overlaps and dynamic travel buffers.
// Filters by gender preference and returns them ordered by rating and volume priority.
func (s *therapistMatchingService) FindAvailableTherapistsForServiceWithTime(
	ctx context.Context,
	clientID int64,
	serviceID int64,
	genderPreference string,
	pressurePreference string,
	scheduledStart time.Time,
	durationMinutes int,
	lat *float64,
	lng *float64,
) ([]model.TherapistProfile, error) {
	if serviceID <= 0 {
		return nil, fmt.Errorf("invalid service_id: must be positive")
	}

	// Validate gender preference
	validGenders := map[string]bool{
		"":       true,
		"any":    true,
		"male":   true,
		"female": true,
	}
	if !validGenders[genderPreference] {
		return nil, fmt.Errorf("invalid gender preference: must be 'male', 'female', or 'any'")
	}

	therapists, err := s.therapistRepo.FindAvailableByServiceWithTime(ctx, clientID, serviceID, genderPreference, pressurePreference, scheduledStart, durationMinutes, lat, lng)
	if err != nil {
		return nil, fmt.Errorf("failed to find available therapists: %w", err)
	}

	if len(therapists) == 0 {
		return []model.TherapistProfile{}, nil
	}

	// Apply fairness sorting using shared helper (refactored from TODO)
	return s.applyFairnessSort(ctx, therapists), nil
}

// FindNearbyAvailableTherapists finds available therapists within a radius using geospatial queries
// Pre-computes and returns closest, highest-rated therapists first
func (s *therapistMatchingService) FindNearbyAvailableTherapists(
	ctx context.Context,
	clientID int64,
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
		clientID,
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
