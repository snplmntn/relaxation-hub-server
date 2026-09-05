package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

const (
	BookingAvailabilityModeSingle = "single"
	BookingAvailabilityModeTandem = "tandem"
	BookingAvailabilityModeGroup  = "group"
)

type BookingAvailabilitySession struct {
	ServiceIDs         []int64 `json:"service_ids"`
	DurationMinutes    int     `json:"duration_minutes"`
	GenderPreference   string  `json:"gender_preference"`
	PressurePreference string  `json:"pressure_preference"`
	TherapistID        *int64  `json:"therapist_id,omitempty"`
}

type BookingAvailabilityRequest struct {
	Mode           string                       `json:"mode"`
	AddressID      int64                        `json:"address_id"`
	ScheduledStart string                       `json:"scheduled_start"`
	Sessions       []BookingAvailabilitySession `json:"sessions"`
}

type BookingAvailabilityResult struct {
	Available bool   `json:"available"`
	Note      string `json:"note"`
}

type bookingAvailabilityMatcher interface {
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
}

type bookingAvailabilityAddressStore interface {
	GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error)
}

type BookingAvailabilityService struct {
	matcher   bookingAvailabilityMatcher
	addresses bookingAvailabilityAddressStore
	now       func() time.Time
}

func NewBookingAvailabilityService(
	matcher bookingAvailabilityMatcher,
	addresses bookingAvailabilityAddressStore,
) *BookingAvailabilityService {
	return &BookingAvailabilityService{
		matcher:   matcher,
		addresses: addresses,
		now:       time.Now,
	}
}

func (s *BookingAvailabilityService) Check(
	ctx context.Context,
	clientID int64,
	req *BookingAvailabilityRequest,
) (*BookingAvailabilityResult, error) {
	start, err := validateBookingAvailabilityRequest(req, s.now())
	if err != nil {
		return nil, err
	}

	address, err := s.addresses.GetByID(ctx, req.AddressID, clientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NewValidationError("invalid_address", "Choose a valid service address.", nil)
		}
		return nil, fmt.Errorf("load booking address: %w", err)
	}
	if address.IsDisabled || address.DisabledAt != nil {
		return nil, NewValidationError("disabled_address", "Choose an enabled service address.", nil)
	}

	var available bool
	switch req.Mode {
	case BookingAvailabilityModeSingle:
		candidates, err := s.sessionCandidates(ctx, clientID, req.Sessions[0], start, address)
		if err != nil {
			return nil, err
		}
		available = len(candidates) > 0
	case BookingAvailabilityModeTandem:
		candidateSets := make([]map[int64]struct{}, 0, len(req.Sessions))
		for _, session := range req.Sessions {
			candidates, err := s.sessionCandidates(ctx, clientID, session, start, address)
			if err != nil {
				return nil, err
			}
			candidateSets = append(candidateSets, candidates)
		}
		available = hasDistinctCandidateAssignment(candidateSets)
	case BookingAvailabilityModeGroup:
		var shared map[int64]struct{}
		sessionStart := start
		for _, session := range req.Sessions {
			candidates, err := s.sessionCandidates(ctx, clientID, session, sessionStart, address)
			if err != nil {
				return nil, err
			}
			shared = intersectCandidateIDs(shared, candidates)
			sessionStart = sessionStart.Add(time.Duration(session.DurationMinutes) * time.Minute)
		}
		available = len(shared) > 0
	}

	note := "A therapist has enough schedule and travel time for the selected services. Staff will honor the gender preference during assignment."
	if !available {
		note = "No therapist has enough schedule and travel time for all selected services. Try a later start time."
	}
	return &BookingAvailabilityResult{Available: available, Note: note}, nil
}

func validateBookingAvailabilityRequest(req *BookingAvailabilityRequest, now time.Time) (time.Time, error) {
	if req == nil {
		return time.Time{}, NewValidationError("invalid_request", "Booking details are required.", nil)
	}
	if req.Mode != BookingAvailabilityModeSingle &&
		req.Mode != BookingAvailabilityModeTandem &&
		req.Mode != BookingAvailabilityModeGroup {
		return time.Time{}, NewValidationError("invalid_mode", "Choose a valid booking type.", nil)
	}
	if req.AddressID <= 0 {
		return time.Time{}, NewValidationError("invalid_address", "Choose a service address.", nil)
	}
	start, err := time.Parse(time.RFC3339, req.ScheduledStart)
	if err != nil {
		return time.Time{}, NewValidationError("invalid_schedule", "Choose a future date and time.", nil)
	}
	if err := validateCustomerBookingLeadTime(start, now); err != nil {
		return time.Time{}, err
	}
	if (req.Mode == BookingAvailabilityModeSingle && len(req.Sessions) != 1) ||
		(req.Mode != BookingAvailabilityModeSingle && (len(req.Sessions) < 2 || len(req.Sessions) > 6)) {
		return time.Time{}, NewValidationError("invalid_sessions", "Complete every booking session.", nil)
	}
	for _, session := range req.Sessions {
		if len(session.ServiceIDs) == 0 || len(session.ServiceIDs) > maxServicesPerBooking {
			return time.Time{}, NewValidationError("invalid_services", "Choose services for every session.", nil)
		}
		if session.DurationMinutes <= 0 || session.DurationMinutes%15 != 0 {
			return time.Time{}, NewValidationError("invalid_duration", "Choose a valid service duration.", nil)
		}
	}
	return start, nil
}

func (s *BookingAvailabilityService) sessionCandidates(
	ctx context.Context,
	clientID int64,
	session BookingAvailabilitySession,
	start time.Time,
	address *model.Address,
) (map[int64]struct{}, error) {
	var shared map[int64]struct{}
	seenServices := make(map[int64]struct{}, len(session.ServiceIDs))
	for _, serviceID := range session.ServiceIDs {
		if serviceID <= 0 {
			return nil, NewValidationError("invalid_services", "Choose valid services.", nil)
		}
		if _, seen := seenServices[serviceID]; seen {
			continue
		}
		seenServices[serviceID] = struct{}{}

		therapists, err := s.matcher.FindAvailableTherapistsForServiceWithTime(
			ctx,
			clientID,
			serviceID,
			"any",
			session.PressurePreference,
			start,
			session.DurationMinutes,
			address.Latitude,
			address.Longitude,
		)
		if err != nil {
			return nil, fmt.Errorf("check therapist availability: %w", err)
		}
		candidates := make(map[int64]struct{}, len(therapists))
		for _, therapist := range therapists {
			if session.TherapistID == nil || therapist.TherapistID == *session.TherapistID {
				candidates[therapist.TherapistID] = struct{}{}
			}
		}
		shared = intersectCandidateIDs(shared, candidates)
	}
	return shared, nil
}

func intersectCandidateIDs(current, next map[int64]struct{}) map[int64]struct{} {
	if current == nil {
		copyOfNext := make(map[int64]struct{}, len(next))
		for id := range next {
			copyOfNext[id] = struct{}{}
		}
		return copyOfNext
	}
	intersection := make(map[int64]struct{})
	for id := range current {
		if _, ok := next[id]; ok {
			intersection[id] = struct{}{}
		}
	}
	return intersection
}

func hasDistinctCandidateAssignment(candidateSets []map[int64]struct{}) bool {
	assigned := make(map[int64]int)
	var assign func(int, map[int64]bool) bool
	assign = func(sessionIndex int, visited map[int64]bool) bool {
		for therapistID := range candidateSets[sessionIndex] {
			if visited[therapistID] {
				continue
			}
			visited[therapistID] = true
			previousSession, occupied := assigned[therapistID]
			if !occupied || assign(previousSession, visited) {
				assigned[therapistID] = sessionIndex
				return true
			}
		}
		return false
	}
	for sessionIndex := range candidateSets {
		if !assign(sessionIndex, make(map[int64]bool)) {
			return false
		}
	}
	return true
}
