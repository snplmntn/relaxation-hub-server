package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// LogisticsService orchestrates ride creation for therapist bookings
type LogisticsService struct {
	rideService              *RideService
	bookingRepo              repository.BookingRepository
	therapistRepo            repository.TherapistRepository
	addressRepo              repository.AddressRepository
	db                       db.DBTX
	automaticDispatchEnabled bool
}

func NewLogisticsService(
	rideService *RideService,
	bookingRepo repository.BookingRepository,
	therapistRepo repository.TherapistRepository,
	addressRepo repository.AddressRepository,
	db db.DBTX,
) *LogisticsService {
	return &LogisticsService{
		rideService:              rideService,
		bookingRepo:              bookingRepo,
		therapistRepo:            therapistRepo,
		addressRepo:              addressRepo,
		db:                       db,
		automaticDispatchEnabled: true,
	}
}

func (s *LogisticsService) DisableAutomaticDispatch() {
	s.automaticDispatchEnabled = false
}

// HandleBookingAssigned is the main orchestration entry point.
// Called when a therapist is assigned to a booking.
func (s *LogisticsService) HandleBookingAssigned(ctx context.Context, bookingID int64) error {
	if !s.automaticDispatchEnabled {
		return nil
	}

	// Fetch booking details
	booking, err := s.bookingRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("failed to fetch booking: %w", err)
	}

	// Validate that therapist is assigned
	if booking.TherapistID == nil {
		slog.Warn("HandleBookingAssigned called but no therapist assigned", "booking_id", bookingID)
		return nil // Not an error, just skip
	}

	// Same-Day Dispatch Rule: Only dispatch if within 12 hours.
	// Otherwise, let the RiderDispatchWorker pick it up closer to the time.
	if booking.ScheduledStart != nil {
		timeUntilStart := time.Until(*booking.ScheduledStart)
		if timeUntilStart > 12*time.Hour {
			slog.Info("HandleBookingAssigned: skipping immediate dispatch (too early)",
				"booking_id", bookingID,
				"scheduled_start", *booking.ScheduledStart,
				"time_until_start", timeUntilStart)
			return nil
		}
	}

	return s.processRideCreation(ctx, booking)
}

// ForceCreateRide creates rides for a booking regardless of the 12-hour dispatch window.
// Primarily for admin use to pre-assign riders for future bookings.
func (s *LogisticsService) ForceCreateRide(ctx context.Context, bookingID int64) error {
	booking, err := s.bookingRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("failed to fetch booking: %w", err)
	}

	if booking.TherapistID == nil {
		return fmt.Errorf("cannot force create ride: no therapist assigned to booking %d", bookingID)
	}

	return s.processRideCreation(ctx, booking)
}

// processRideCreation contains the core logic for creating outbound and return rides.
func (s *LogisticsService) processRideCreation(ctx context.Context, booking *model.Booking) error {
	// Validate that we have necessary data
	if booking.AddressID == nil {
		slog.Warn("processRideCreation: booking has no address", "booking_id", booking.BookingID)
		return nil
	}

	if booking.ScheduledStart == nil {
		slog.Warn("processRideCreation: booking has no scheduled start", "booking_id", booking.BookingID)
		return nil
	}

	// Create outbound ride (therapist -> client)
	if err := s.createOutboundRide(ctx, booking, false); err != nil {
		slog.Error("Failed to create outbound ride", "booking_id", booking.BookingID, "error", err)
	}

	// Schedule return ride (client -> therapist home/branch)
	if err := s.scheduleReturnRide(ctx, booking, false); err != nil {
		slog.Error("Failed to schedule return ride", "booking_id", booking.BookingID, "error", err)
	}

	return nil
}

// CancelRideForBooking cancels all active rides linked to a booking.
// Called when a booking is cancelled or a therapist is unassigned.
func (s *LogisticsService) CancelRideForBooking(ctx context.Context, bookingID int64) error {
	rides, err := s.rideService.GetRidesByBookingID(ctx, bookingID)
	if err != nil {
		slog.Warn("CancelRideForBooking: failed to fetch rides", "booking_id", bookingID, "error", err)
		return nil // Non-fatal
	}

	for _, ride := range rides {
		// Only cancel rides that are still active (pending, offered, accepted)
		if ride.Status == "pending" || ride.Status == "offered" || ride.Status == "accepted" || ride.Status == "arrived_pickup" {
			if err := s.rideService.CancelRide(ctx, ride.RideID); err != nil {
				slog.Warn("CancelRideForBooking: failed to cancel ride",
					"ride_id", ride.RideID, "booking_id", bookingID, "error", err)
			} else {
				slog.Info("CancelRideForBooking: ride cancelled",
					"ride_id", ride.RideID, "booking_id", bookingID)
			}
		}
	}
	return nil
}

// UpdateRideForBooking handles booking rescheduling by cancelling existing rides
// and re-creating them with updated details.
func (s *LogisticsService) UpdateRideForBooking(ctx context.Context, bookingID int64) error {
	// Cancel existing rides
	if err := s.CancelRideForBooking(ctx, bookingID); err != nil {
		slog.Warn("UpdateRideForBooking: failed to cancel old rides", "booking_id", bookingID, "error", err)
	}

	// Re-create rides with updated booking details
	return s.HandleBookingAssigned(ctx, bookingID)
}

// AssignRiderToBookingLeg explicitly assigns a rider to a specific leg (outbound/return) of a booking.
func (s *LogisticsService) AssignRiderToBookingLeg(ctx context.Context, bookingID int64, riderID int64, rideType string) error {
	booking, err := s.bookingRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("failed to fetch booking: %w", err)
	}

	if booking.TherapistID == nil {
		return NewValidationError("rider_assignment_unavailable", "Assign a therapist before assigning a rider.", map[string]string{"therapist_id": "required"})
	}

	// 1. Check if ride already exists
	var existingRide *model.Ride
	rides, err := s.rideService.GetRidesByBookingID(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("failed to fetch rides for booking %d: %w", bookingID, err)
	}
	for _, r := range rides {
		if r.RideType == rideType && isAssignableRideForManualRiderAssignment(r) {
			existingRide = &r
			break
		}
	}

	if existingRide == nil {
		if err := s.createRideForBookingLeg(ctx, booking, rideType); err != nil {
			var validationErr *ValidationError
			if errors.As(err, &validationErr) {
				return validationErr
			}
			return fmt.Errorf("failed to create ride for leg %s: %w", rideType, err)
		}
		// Fetch again after creation
		rides, err = s.rideService.GetRidesByBookingID(ctx, bookingID)
		if err != nil {
			return fmt.Errorf("failed to fetch rides after creating leg %s: %w", rideType, err)
		}
		for _, r := range rides {
			if r.RideType == rideType && isAssignableRideForManualRiderAssignment(r) {
				existingRide = &r
				break
			}
		}
	}

	if existingRide == nil {
		return NewValidationError("ride_resolution_failed", fmt.Sprintf("Could not resolve the %s ride for this booking.", rideType), map[string]string{"ride_type": rideType})
	}

	// 2. Force assign the rider
	return s.rideService.ForceAssignRider(ctx, existingRide.RideID, riderID)
}

func (s *LogisticsService) createRideForBookingLeg(ctx context.Context, booking *model.Booking, rideType string) error {
	// Manual rider assignment: the admin is picking the rider directly, so map
	// coordinates are not required to create the ride leg. Missing coordinates
	// default to 0,0 (unknown) instead of blocking the assignment.
	switch rideType {
	case "outbound":
		return s.createOutboundRide(ctx, booking, true)
	case "return":
		return s.scheduleReturnRide(ctx, booking, true)
	default:
		return fmt.Errorf("unsupported ride type %q", rideType)
	}
}

func isAssignableRideForManualRiderAssignment(ride model.Ride) bool {
	switch ride.Status {
	case "cancelled", "completed":
		return false
	default:
		return true
	}
}

func (s *LogisticsService) createOutboundRide(ctx context.Context, booking *model.Booking, allowMissingCoordinates bool) error {
	// Get therapist pickup location (branch or home)
	pickupLat, pickupLong, pickupAddr, err := s.getTherapistPickupLocation(ctx, *booking.TherapistID, allowMissingCoordinates)
	if err != nil {
		return fmt.Errorf("failed to get therapist location: %w", err)
	}

	// Get client dropoff location
	dropoffLat, dropoffLong, dropoffAddr, err := s.getClientLocation(ctx, *booking.AddressID, allowMissingCoordinates)
	if err != nil {
		return fmt.Errorf("failed to get client location: %w", err)
	}

	// Create the ride
	ride := &model.Ride{
		PassengerID:    *booking.TherapistID, // Therapist is the passenger
		BookingID:      &booking.BookingID,
		RideType:       "outbound",
		PickupLat:      pickupLat,
		PickupLong:     pickupLong,
		PickupAddress:  pickupAddr,
		DropoffLat:     dropoffLat,
		DropoffLong:    dropoffLong,
		DropoffAddress: dropoffAddr,
	}

	createdRide, err := s.rideService.RequestRide(ctx, ride)
	if err != nil {
		return fmt.Errorf("failed to request outbound ride: %w", err)
	}

	slog.Info("Outbound ride created",
		"booking_id", booking.BookingID,
		"ride_id", createdRide.RideID,
		"therapist_id", *booking.TherapistID,
	)

	return nil
}

func (s *LogisticsService) scheduleReturnRide(ctx context.Context, booking *model.Booking, allowMissingCoordinates bool) error {
	// Calculate return ride time: scheduled_start + duration + 30min buffer
	bufferMinutes := 30
	returnTime := booking.ScheduledStart.Add(time.Duration(booking.DurationMinutes+bufferMinutes) * time.Minute)

	returnState, err := s.buildReturnRideOptions(ctx, booking, returnTime)
	if err != nil {
		return fmt.Errorf("failed to build return destination options: %w", err)
	}
	returnOption, ok := selectedReturnRideOption(returnState)
	dropoffLat, dropoffLong := 0.0, 0.0
	dropoffAddr := ""
	if ok && returnOption.Latitude != nil && returnOption.Longitude != nil {
		dropoffLat = *returnOption.Latitude
		dropoffLong = *returnOption.Longitude
		dropoffAddr = returnOption.Address
	} else if !allowMissingCoordinates {
		return NewValidationError("return_destination_unavailable", "Therapist needs a next booking, branch, or home address with map coordinates before assigning a return rider.", map[string]string{"ride_type": "return"})
	} else if ok {
		// Manual assignment: keep whatever address we have even without coordinates.
		dropoffAddr = returnOption.Address
	}

	// Get client location (pickup for return ride)
	pickupLat, pickupLong, pickupAddr, err := s.getClientLocation(ctx, *booking.AddressID, allowMissingCoordinates)
	if err != nil {
		return fmt.Errorf("failed to get client location: %w", err)
	}

	// Create the return ride with future scheduled time
	ride := &model.Ride{
		PassengerID:    *booking.TherapistID,
		BookingID:      &booking.BookingID,
		RideType:       "return",
		PickupLat:      pickupLat, // Client location
		PickupLong:     pickupLong,
		PickupAddress:  pickupAddr,
		DropoffLat:     dropoffLat,
		DropoffLong:    dropoffLong,
		DropoffAddress: dropoffAddr,
		ScheduledFor:   &returnTime,
		// Note: We're creating the ride now but it should be matched closer to returnTime
		// For MVP, we create it immediately. Future: implement scheduled ride matching
	}

	createdRide, err := s.rideService.RequestRide(ctx, ride)
	if err != nil {
		return fmt.Errorf("failed to request return ride: %w", err)
	}

	slog.Info("Return ride scheduled",
		"booking_id", booking.BookingID,
		"ride_id", createdRide.RideID,
		"scheduled_for", returnTime.Format(time.RFC3339),
	)

	return nil
}

func (s *LogisticsService) buildReturnRideOptions(ctx context.Context, booking *model.Booking, after time.Time) (*model.ReturnRideState, error) {
	if booking.TherapistID == nil {
		return nil, fmt.Errorf("booking has no therapist")
	}

	profile, err := s.therapistRepo.GetProfile(ctx, *booking.TherapistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get therapist profile: %w", err)
	}

	nextOption, err := s.buildNextBookingReturnOption(ctx, *booking.TherapistID, booking.BookingID, after)
	if err != nil {
		return nil, err
	}

	options := []model.ReturnRideOption{
		nextOption,
		s.buildBranchReturnOption(ctx, profile),
		s.buildHomeReturnOption(ctx, profile),
	}

	state := &model.ReturnRideState{Options: options}
	for _, option := range options {
		if option.Available {
			state.Destination = option.Destination
			state.DestinationLabel = option.Label
			state.Ready = true
			break
		}
	}

	return state, nil
}

func (s *LogisticsService) buildNextBookingReturnOption(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (model.ReturnRideOption, error) {
	option := model.ReturnRideOption{Destination: model.ReturnRideDestinationNextBooking, Label: "Next booking"}

	next, err := s.bookingRepo.FindNextReturnDestinationBooking(ctx, therapistID, excludeBookingID, after)
	if err != nil {
		return option, fmt.Errorf("find next return destination booking: %w", err)
	}
	if next == nil || next.Booking == nil || next.Address == nil || next.Address.Latitude == nil || next.Address.Longitude == nil {
		option.DisabledReason = "No later booking with a mapped address"
		return option, nil
	}

	option.Address = formatAddress(next.Address)
	option.Latitude = next.Address.Latitude
	option.Longitude = next.Address.Longitude
	option.BookingID = &next.Booking.BookingID
	option.Available = true
	return option, nil
}

func (s *LogisticsService) buildBranchReturnOption(ctx context.Context, profile *model.TherapistProfile) model.ReturnRideOption {
	option := model.ReturnRideOption{Destination: model.ReturnRideDestinationBranch, Label: "Branch"}
	if profile.BranchID == nil || *profile.BranchID <= 0 {
		option.DisabledReason = "Therapist has no branch"
		return option
	}

	branch, err := s.getBranchLocation(ctx, *profile.BranchID)
	if err != nil || branch.Latitude == nil || branch.Longitude == nil {
		option.DisabledReason = "Branch address has no coordinates"
		return option
	}

	option.Address = formatBranchAddress(branch)
	option.Latitude = branch.Latitude
	option.Longitude = branch.Longitude
	option.Available = true
	return option
}

func (s *LogisticsService) buildHomeReturnOption(ctx context.Context, profile *model.TherapistProfile) model.ReturnRideOption {
	option := model.ReturnRideOption{Destination: model.ReturnRideDestinationHome, Label: "Home"}
	if profile.HomeAddressID == nil || *profile.HomeAddressID <= 0 {
		option.DisabledReason = "Therapist has no home address"
		return option
	}

	address, err := s.addressRepo.GetByIDUnsafe(ctx, *profile.HomeAddressID)
	if err != nil || address == nil || address.Latitude == nil || address.Longitude == nil {
		option.DisabledReason = "Home address has no coordinates"
		return option
	}

	option.Address = formatAddress(address)
	option.Latitude = address.Latitude
	option.Longitude = address.Longitude
	option.Available = true
	return option
}

func selectedReturnRideOption(state *model.ReturnRideState) (model.ReturnRideOption, bool) {
	if state == nil {
		return model.ReturnRideOption{}, false
	}
	for _, option := range state.Options {
		if option.Destination == state.Destination && option.Available {
			return option, true
		}
	}
	return model.ReturnRideOption{}, false
}

// getTherapistPickupLocation resolves therapist's starting location
// Priority: home_address_id > branch_id
func (s *LogisticsService) getTherapistPickupLocation(ctx context.Context, therapistID int64, allowMissingCoordinates bool) (lat, long float64, addr string, err error) {
	profile, err := s.therapistRepo.GetProfile(ctx, therapistID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get therapist profile: %w", err)
	}

	// Best-effort address string to keep even when coordinates are unavailable.
	fallbackAddr := ""

	// Try home address first (if set)
	// Note: home_address_id is added in migration 043, may be NULL for existing therapists
	if profile.HomeAddressID != nil && *profile.HomeAddressID > 0 {
		address, err := s.addressRepo.GetByIDUnsafe(ctx, *profile.HomeAddressID)
		if err == nil && address != nil {
			if address.Latitude != nil && address.Longitude != nil {
				return float64(*address.Latitude), float64(*address.Longitude),
					formatAddress(address), nil
			}
			if fallbackAddr == "" {
				fallbackAddr = formatAddress(address)
			}
		}
	}

	// Fallback to branch location
	if profile.BranchID != nil && *profile.BranchID > 0 {
		branch, err := s.getBranchLocation(ctx, *profile.BranchID)
		if err == nil && branch != nil {
			if branch.Latitude != nil && branch.Longitude != nil {
				return float64(*branch.Latitude), float64(*branch.Longitude),
					formatBranchAddress(branch), nil
			}
			if fallbackAddr == "" {
				fallbackAddr = formatBranchAddress(branch)
			}
		}
	}

	if allowMissingCoordinates {
		return 0, 0, fallbackAddr, nil
	}

	return 0, 0, "", NewValidationError("therapist_pickup_unavailable", "Therapist needs a home address or branch with map coordinates before assigning an outbound rider.", map[string]string{"therapist_id": fmt.Sprintf("%d", therapistID)})
}

// getClientLocation fetches client address coordinates
func (s *LogisticsService) getClientLocation(ctx context.Context, addressID int64, allowMissingCoordinates bool) (lat, long float64, addr string, err error) {
	address, err := s.addressRepo.GetByIDUnsafe(ctx, addressID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get client address: %w", err)
	}

	if address == nil || address.Latitude == nil || address.Longitude == nil {
		if allowMissingCoordinates {
			fallbackAddr := ""
			if address != nil {
				fallbackAddr = formatAddress(address)
			}
			return 0, 0, fallbackAddr, nil
		}
		return 0, 0, "", NewValidationError("client_address_unmapped", "Client address needs map coordinates before assigning a rider.", map[string]string{"address_id": fmt.Sprintf("%d", addressID)})
	}

	return float64(*address.Latitude), float64(*address.Longitude), formatAddress(address), nil
}

// getBranchLocation fetches branch coordinates
func (s *LogisticsService) getBranchLocation(ctx context.Context, branchID int64) (*model.Branch, error) {
	var branch model.Branch
	query := `
		SELECT branch_id, branch_name, address_line, city, latitude, longitude
		FROM branches
		WHERE branch_id = $1 AND deleted_at IS NULL
	`
	err := s.db.QueryRow(ctx, query, branchID).Scan(
		&branch.BranchID,
		&branch.BranchName,
		&branch.AddressLine,
		&branch.City,
		&branch.Latitude,
		&branch.Longitude,
	)
	if err != nil {
		return nil, err
	}
	return &branch, nil
}

// formatAddress creates a human-readable address string
func formatAddress(addr *model.Address) string {
	if addr.Label != "" {
		return fmt.Sprintf("%s (%s, %s)", addr.Label, addr.Street, addr.City)
	}
	return fmt.Sprintf("%s, %s", addr.Street, addr.City)
}

// formatBranchAddress creates a human-readable branch address
func formatBranchAddress(branch *model.Branch) string {
	if branch.AddressLine != nil {
		return fmt.Sprintf("%s (%s)", branch.BranchName, *branch.AddressLine)
	}
	return branch.BranchName
}
