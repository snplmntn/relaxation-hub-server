package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// LogisticsService orchestrates ride creation for therapist bookings
type LogisticsService struct {
	rideService   *RideService
	bookingRepo   repository.BookingRepository
	therapistRepo repository.TherapistRepository
	addressRepo   repository.AddressRepository
	db            db.DBTX
}

func NewLogisticsService(
	rideService *RideService,
	bookingRepo repository.BookingRepository,
	therapistRepo repository.TherapistRepository,
	addressRepo repository.AddressRepository,
	db db.DBTX,
) *LogisticsService {
	return &LogisticsService{
		rideService:   rideService,
		bookingRepo:   bookingRepo,
		therapistRepo: therapistRepo,
		addressRepo:   addressRepo,
		db:            db,
	}
}

// HandleBookingAssigned is the main orchestration entry point.
// Called when a therapist is assigned to a booking.
func (s *LogisticsService) HandleBookingAssigned(ctx context.Context, bookingID int64) error {
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
	if err := s.createOutboundRide(ctx, booking); err != nil {
		slog.Error("Failed to create outbound ride", "booking_id", booking.BookingID, "error", err)
	}

	// Schedule return ride (client -> therapist home/branch)
	if err := s.scheduleReturnRide(ctx, booking); err != nil {
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

func (s *LogisticsService) createOutboundRide(ctx context.Context, booking *model.Booking) error {
	// Get therapist pickup location (branch or home)
	pickupLat, pickupLong, pickupAddr, err := s.getTherapistPickupLocation(ctx, *booking.TherapistID)
	if err != nil {
		return fmt.Errorf("failed to get therapist location: %w", err)
	}

	// Get client dropoff location
	dropoffLat, dropoffLong, dropoffAddr, err := s.getClientLocation(ctx, *booking.AddressID)
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

func (s *LogisticsService) scheduleReturnRide(ctx context.Context, booking *model.Booking) error {
	// Calculate return ride time: scheduled_start + duration + 30min buffer
	bufferMinutes := 30
	returnTime := booking.ScheduledStart.Add(time.Duration(booking.DurationMinutes+bufferMinutes) * time.Minute)

	// Get therapist return destination (branch or home)
	returnLat, returnLong, returnAddr, err := s.getTherapistPickupLocation(ctx, *booking.TherapistID)
	if err != nil {
		return fmt.Errorf("failed to get therapist return location: %w", err)
	}

	// Get client location (pickup for return ride)
	pickupLat, pickupLong, pickupAddr, err := s.getClientLocation(ctx, *booking.AddressID)
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
		DropoffLat:     returnLat, // Therapist home/branch
		DropoffLong:    returnLong,
		DropoffAddress: returnAddr,
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

// getTherapistPickupLocation resolves therapist's starting location
// Priority: home_address_id > branch_id
func (s *LogisticsService) getTherapistPickupLocation(ctx context.Context, therapistID int64) (lat, long float64, addr string, err error) {
	profile, err := s.therapistRepo.GetProfile(ctx, therapistID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get therapist profile: %w", err)
	}

	// Try home address first (if set)
	// Note: home_address_id is added in migration 043, may be NULL for existing therapists
	if profile.HomeAddressID != nil && *profile.HomeAddressID > 0 {
		address, err := s.addressRepo.GetByIDUnsafe(ctx, *profile.HomeAddressID)
		if err == nil && address.Latitude != nil && address.Longitude != nil {
			return float64(*address.Latitude), float64(*address.Longitude),
				formatAddress(address), nil
		}
	}

	// Fallback to branch location
	if profile.BranchID != nil && *profile.BranchID > 0 {
		branch, err := s.getBranchLocation(ctx, *profile.BranchID)
		if err == nil && branch.Latitude != nil && branch.Longitude != nil {
			return float64(*branch.Latitude), float64(*branch.Longitude),
				formatBranchAddress(branch), nil
		}
	}

	return 0, 0, "", fmt.Errorf("therapist has no valid pickup location (no home address or branch)")
}

// getClientLocation fetches client address coordinates
func (s *LogisticsService) getClientLocation(ctx context.Context, addressID int64) (lat, long float64, addr string, err error) {
	address, err := s.addressRepo.GetByIDUnsafe(ctx, addressID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get client address: %w", err)
	}

	if address.Latitude == nil || address.Longitude == nil {
		return 0, 0, "", fmt.Errorf("client address has no coordinates")
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
