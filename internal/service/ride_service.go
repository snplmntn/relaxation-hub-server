package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrNoRidersAvailable = errors.New("no riders available nearby")
)

// BookingStatusUpdater is a minimal interface for updating booking status
// from ride events. Avoids circular dependency with BookingService.
type BookingStatusUpdater interface {
	UpdateStatusFromRide(ctx context.Context, bookingID int64, status string) error
}

type RideService struct {
	repo            repository.RideRepository
	offerRepo       repository.RideOfferRepository
	pricingService  *RidePricingService
	matchingService *RideMatchingService
	notificationSvc *NotificationService
	messageService  *MessageService
	geocoder        Geocoder
	bookingUpdater  BookingStatusUpdater
	db              db.DBTX
}

func NewRideService(repo repository.RideRepository, offerRepo repository.RideOfferRepository, pricing *RidePricingService, matching *RideMatchingService, db db.DBTX) *RideService {
	return &RideService{
		repo:            repo,
		offerRepo:       offerRepo,
		pricingService:  pricing,
		matchingService: matching,
		notificationSvc: nil,
		geocoder:        nil,
		bookingUpdater:  nil,
		db:              db,
	}
}

func (s *RideService) SetGeocoder(g Geocoder) {
	s.geocoder = g
}

// SetNotificationService allows injecting the notification service after creation
func (s *RideService) SetNotificationService(svc *NotificationService) {
	s.notificationSvc = svc
}

// SetMessageService allows injecting the message service for auto-conversation creation
func (s *RideService) SetMessageService(svc *MessageService) {
	s.messageService = svc
}

// SetBookingUpdater allows injecting the booking status updater for ride→booking sync
func (s *RideService) SetBookingUpdater(u BookingStatusUpdater) {
	s.bookingUpdater = u
}

func (s *RideService) RequestRide(ctx context.Context, ride *model.Ride) (*model.Ride, error) {
	// 1. Calculate Price
	config, err := s.pricingService.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing config: %w", err)
	}

	// Calculate distance server-side for security (prevent client manipulation)
	distance, err := s.calculateDistance(ctx,
		ride.PickupLat, ride.PickupLong,
		ride.DropoffLat, ride.DropoffLong)
	if err != nil {
		// Fallback to client distance if PostGIS calculation fails
		if ride.DistanceKm != nil {
			distance = *ride.DistanceKm
		} else {
			return nil, fmt.Errorf("distance calculation failed and no client distance provided: %w", err)
		}
	}
	ride.DistanceKm = &distance

	pricing := s.pricingService.CalculateFare(distance, config)
	ride.Pricing = pricing
	snapshot, _ := json.Marshal(pricing)
	ride.PricingSnapshot = snapshot
	ride.Status = "pending"

	// 2. Persist Ride
	if err := s.repo.Create(ctx, ride); err != nil {
		return nil, err
	}

	// 3. Find Match (Broadcast Mode)
	// Radius 5km, schedule-aware filtering
	riders, err := s.matchingService.FindNearbyRiders(ctx, ride.PickupLat, ride.PickupLong, 5.0, ride.ScheduledFor)
	if err != nil {
		// Log error but return created ride
		return ride, nil
	}

	if len(riders) > 0 {
		// Broadcast to all nearby riders
		// We DO NOT assign to a specific rider immediately.
		// Status remains 'pending'.

		// Send notifications to ALL nearby riders
		if s.notificationSvc != nil {
			// Prepare payload
			data := map[string]string{
				"ride_id":         fmt.Sprintf("%d", ride.RideID),
				"pickup_address":  ride.PickupAddress,
				"dropoff_address": ride.DropoffAddress,
				"pickup_lat":      fmt.Sprintf("%f", ride.PickupLat),
				"pickup_long":     fmt.Sprintf("%f", ride.PickupLong),
				"dropoff_lat":     fmt.Sprintf("%f", ride.DropoffLat),
				"dropoff_long":    fmt.Sprintf("%f", ride.DropoffLong),
			}
			if ride.Pricing != nil {
				data["estimated_fare"] = fmt.Sprintf("%.2f", ride.Pricing.FinalFare)
			}
			if ride.DistanceKm != nil {
				data["distance_km"] = fmt.Sprintf("%.1f", *ride.DistanceKm)
			}

			title := "🚗 New Ride Offer!"
			message := fmt.Sprintf("Pickup: %s", ride.PickupAddress)
			if ride.Pricing != nil {
				message += fmt.Sprintf("\nFare: ₱%.2f", ride.Pricing.FinalFare)
			}

			// Deduplication: skip riders who already have a pending offer for this ride
			existingOfferSet := make(map[int64]bool)
			if s.offerRepo != nil {
				if existing, err := s.offerRepo.GetActiveByRideID(ctx, ride.RideID); err == nil {
					for _, eo := range existing {
						existingOfferSet[eo.RiderID] = true
					}
				}
			}

			for _, r := range riders {
				if existingOfferSet[r.RiderID] {
					continue // Skip: already has pending offer
				}
				// Persist offer in ride_offers table
				if s.offerRepo != nil {
					offer := &model.RideOffer{
						RideID:    ride.RideID,
						RiderID:   r.RiderID,
						ExpiresAt: time.Now().Add(repository.DefaultRideOfferTTL),
					}
					if err := s.offerRepo.Create(ctx, offer); err != nil {
						slog.Warn("failed to persist ride offer", "ride_id", ride.RideID, "rider_id", r.RiderID, "error", err)
					}
				}
				go s.notificationSvc.SendPushDirect(context.Background(), r.UserID, "ride_offer", title, message, data)
				// Also Broadcast via WebSocket if online
				go broadcaster.BroadcastToUser(r.UserID, "ride_offer", ride)
			}
		}
	}

	return ride, nil
}

// calculateDistance uses PostGIS to calculate distance between two coordinates
// Returns distance in kilometers
func (s *RideService) calculateDistance(ctx context.Context, lat1, lng1, lat2, lng2 float64) (float64, error) {
	query := `
		SELECT ST_Distance(
			ST_MakePoint($1, $2)::geography,
			ST_MakePoint($3, $4)::geography
		) / 1000.0 AS distance_km
	`

	var distanceKm float64
	// Note: PostGIS ST_MakePoint takes (longitude, latitude)
	err := s.db.QueryRow(ctx, query, lng1, lat1, lng2, lat2).Scan(&distanceKm)
	if err != nil {
		return 0, fmt.Errorf("calc distance error: %w", err)
	}
	return distanceKm, nil
}

func (s *RideService) GetRiderOffers(ctx context.Context, riderID int64) ([]model.Ride, error) {
	// If offerRepo is available, fetch active broadcast offers
	if s.offerRepo != nil {
		offers, err := s.offerRepo.GetActiveForRider(ctx, riderID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch rider offers: %w", err)
		}

		var rides []model.Ride
		for _, offer := range offers {
			ride, err := s.repo.GetByID(ctx, offer.RideID)
			if err != nil {
				slog.Warn("GetRiderOffers: failed to fetch ride details", "ride_id", offer.RideID, "error", err)
				continue
			}
			// Only include pending rides (in case status changed but offer not yet expired/updated)
			if ride.Status == "pending" {
				rides = append(rides, *ride)
			}
		}
		return rides, nil
	}

	// Fallback to old behavior (though widely deprecated for broadcast model)
	return s.repo.GetRidesForRiderByStatus(ctx, riderID, "offered")
}

// GetAvailableRides returns rides that are pending and near the rider
func (s *RideService) GetAvailableRides(ctx context.Context, riderID int64, lat, long, radiusKm float64) ([]model.Ride, error) {
	return s.repo.GetAvailableRidesNear(ctx, lat, long, radiusKm)
}

func (s *RideService) AcceptRide(ctx context.Context, rideID, riderID int64) error {
	// Atomic claim: locks row, verifies availability, assigns rider, sets 'accepted'
	if err := s.repo.ClaimRide(ctx, rideID, riderID); err != nil {
		return err
	}

	// Expire all other pending offers for this ride
	if s.offerRepo != nil {
		if _, err := s.offerRepo.ExpireOffersForRide(ctx, rideID); err != nil {
			slog.Warn("failed to expire other ride offers", "ride_id", rideID, "error", err)
		}
		// Mark this rider's offer as accepted
		offer, err := s.offerRepo.GetByRiderAndRide(ctx, riderID, rideID)
		if err == nil && offer != nil {
			_ = s.offerRepo.UpdateStatus(ctx, offer.OfferID, model.RideOfferStatusAccepted)
		}
	}

	// Notify passenger that a rider has been assigned
	ride, err := s.repo.GetByID(ctx, rideID)
	if err == nil {
		_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:accepted", map[string]any{
			"ride_id":  rideID,
			"rider_id": riderID,
			"status":   "accepted",
		})

		// Auto-create Rider↔Therapist conversation + system message (best-effort)
		if s.messageService != nil {
			go func() {
				req := &model.CreateConversationRequest{
					ParticipantIDs: []int64{ride.PassengerID},
				}
				conv, err := s.messageService.CreateConversation(context.Background(), riderID, req)
				if err != nil {
					slog.Warn("AcceptRide: auto-conversation creation failed", "rider_id", riderID, "passenger_id", ride.PassengerID, "error", err)
					return
				}
				slog.Debug("AcceptRide: conversation created", "rider_id", riderID, "passenger_id", ride.PassengerID)
				_ = s.messageService.SendSystemMessage(context.Background(), conv.ConversationID, "Rider has accepted the ride request.")
			}()
		}
	}

	return nil
}

func (s *RideService) UpdateRideStatus(ctx context.Context, rideID, riderID int64, status string) error {
	if err := s.repo.UpdateStatus(ctx, rideID, status); err != nil {
		return err
	}

	// Fetch ride for broadcast and booking sync
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		slog.Warn("UpdateRideStatus: failed to fetch ride for broadcast", "ride_id", rideID, "error", err)
		return nil // status already updated, broadcast failure is non-fatal
	}

	// Broadcast ride status to rider and passenger
	// Must send FULL ride object because Rider App replaces local state with payload.
	go func() {
		_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:status_updated", ride)
		if ride.RiderID != nil {
			_ = broadcaster.BroadcastToUser(*ride.RiderID, "ride:status_updated", ride)
		}
	}()

	// Send system message for key status transitions
	if s.messageService != nil && ride.RiderID != nil {
		var sysMsg string
		switch status {
		case "arrived_pickup":
			sysMsg = "Rider has arrived at the pickup point."
		case "in_progress":
			sysMsg = "Ride started. Heading to destination."
		case "arrived_dropoff":
			sysMsg = "Rider has arrived at the drop-off location."
		case "completed":
			sysMsg = "Ride completed. Thank you!"
		}
		if sysMsg != "" {
			go s.sendRideSystemMessage(ride.PassengerID, *ride.RiderID, sysMsg)
		}
	}

	// Sync ride status to booking status
	if s.bookingUpdater != nil && ride.BookingID != nil {
		var bookingStatus string
		switch status {
		case "arrived_pickup":
			bookingStatus = "on_the_way"
		case "arrived_dropoff":
			bookingStatus = "arrived"
		}
		if bookingStatus != "" {
			go func() {
				if err := s.bookingUpdater.UpdateStatusFromRide(context.Background(), *ride.BookingID, bookingStatus); err != nil {
					slog.Error("UpdateRideStatus: failed to sync booking status",
						"ride_id", rideID, "booking_id", *ride.BookingID,
						"ride_status", status, "booking_status", bookingStatus, "error", err)
				} else {
					slog.Info("UpdateRideStatus: synced booking status",
						"ride_id", rideID, "booking_id", *ride.BookingID,
						"booking_status", bookingStatus)
				}
			}()
		}
	}

	return nil
}

// sendRideSystemMessage finds/creates the conversation between rider and passenger and sends a system message.
func (s *RideService) sendRideSystemMessage(passengerID, riderID int64, content string) {
	ctx := context.Background()
	req := &model.CreateConversationRequest{ParticipantIDs: []int64{passengerID}}
	conv, err := s.messageService.CreateConversation(ctx, riderID, req)
	if err != nil {
		slog.Warn("sendRideSystemMessage: conversation lookup failed", "rider_id", riderID, "passenger_id", passengerID, "error", err)
		return
	}
	_ = s.messageService.SendSystemMessage(ctx, conv.ConversationID, content)
}

func (s *RideService) UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error {
	if err := s.repo.UpdateRiderLocation(ctx, riderID, lat, long); err != nil {
		return err
	}

	// Check for active ride to broadcast location
	ride, err := s.repo.GetActiveRideByRiderID(ctx, riderID)
	if err == nil && ride != nil {
		// Broadcast to passenger (Therapist)
		_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:location_update", map[string]any{
			"ride_id":    ride.RideID,
			"booking_id": ride.BookingID,
			"lat":        lat,
			"long":       long,
			"rider_id":   riderID,
		})
	}

	return nil
}

func (s *RideService) UpdateRiderLocationByUserID(ctx context.Context, userID int64, lat, long float64) error {
	rider, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			// Lazily create profile if not found
			if createErr := s.repo.CreateRiderProfile(ctx, userID, "Unspecified", "PENDING"); createErr != nil {
				return fmt.Errorf("failed to lazily create rider profile: %w", createErr)
			}
			rider, err = s.repo.GetRiderProfile(ctx, userID)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return s.UpdateRiderLocation(ctx, rider.RiderID, lat, long)
}

func (s *RideService) ToggleOnlineStatus(ctx context.Context, userID int64, isOnline bool) error {
	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			// Lazily create profile if not found
			if createErr := s.repo.CreateRiderProfile(ctx, userID, "Unspecified", "PENDING"); createErr != nil {
				return fmt.Errorf("failed to lazily create rider profile: %w", createErr)
			}
			profile, err = s.repo.GetRiderProfile(ctx, userID)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return s.repo.UpdateRiderStatus(ctx, profile.RiderID, isOnline)
}

func (s *RideService) GetRiderOffersByUserID(ctx context.Context, userID int64) ([]model.Ride, error) {
	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return []model.Ride{}, nil
		}
		return nil, fmt.Errorf("failed to get rider profile: %w", err)
	}
	return s.GetRiderOffers(ctx, profile.RiderID)
}

func (s *RideService) CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error {
	// Check if already exists
	_, err := s.repo.GetRiderProfile(ctx, userID)
	if err == nil {
		return errors.New("rider profile already exists")
	}
	// Create
	return s.repo.CreateRiderProfile(ctx, userID, vehicleType, licensePlate)
}

func (s *RideService) GetActiveRideForRider(ctx context.Context, userID int64) (*model.Ride, error) {
	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	// Query repo for active ride
	return s.repo.GetActiveRideByRiderID(ctx, profile.RiderID)
}

func (s *RideService) GetRideHistoryForRider(ctx context.Context, userID int64, status string, limit, offset int) ([]model.Ride, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return []model.Ride{}, false, nil
		}
		return nil, false, err
	}

	// Fetch one extra record to compute has_more without a separate COUNT query.
	rides, err := s.repo.GetRidesForRider(ctx, profile.RiderID, status, limit+1, offset)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(rides) > limit
	if hasMore {
		rides = rides[:limit]
	}

	return rides, hasMore, nil
}

func (s *RideService) UpdateRiderProfile(ctx context.Context, userID int64, updates map[string]interface{}) error {
	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.UpdateRiderProfile(ctx, profile.RiderID, updates)
}

func (s *RideService) DeclineRide(ctx context.Context, rideID, userID int64) error {
	slog.Info("ride declined", "ride_id", rideID, "rider_user_id", userID)

	// Persist decline if offer repo is available
	if s.offerRepo != nil {
		// Look up the rider profile to get rider_id
		profile, err := s.repo.GetRiderProfile(ctx, userID)
		if err != nil {
			slog.Warn("decline ride: rider profile not found", "user_id", userID, "error", err)
			return nil
		}
		offer, err := s.offerRepo.GetByRiderAndRide(ctx, profile.RiderID, rideID)
		if err != nil {
			slog.Warn("decline ride: offer not found", "ride_id", rideID, "rider_id", profile.RiderID, "error", err)
			return nil
		}
		if err := s.offerRepo.DeclineOffer(ctx, offer.OfferID); err != nil {
			slog.Warn("decline ride: failed to decline offer", "offer_id", offer.OfferID, "error", err)
		}
	}

	return nil
}

// ExpireStaleOffers bulk-expires ride offers past their TTL.
// Called by the dispatch worker on each tick.
func (s *RideService) ExpireStaleOffers(ctx context.Context) {
	if s.offerRepo == nil {
		return
	}
	expired, err := s.offerRepo.ExpireStaleOffers(ctx)
	if err != nil {
		slog.Warn("failed to expire stale ride offers", "error", err)
		return
	}
	if len(expired) > 0 {
		slog.Info("expired stale ride offers", "count", len(expired))
	}
}

const maxRideRetries = 3
const retryBackoffMinutes = 5

// RetryUnmatchedRides picks up pending rides with no active offers and re-broadcasts.
// Called by the dispatch worker each tick.
func (s *RideService) RetryUnmatchedRides(ctx context.Context) {
	rides, err := s.repo.GetUnmatchedRidesForRetry(ctx, retryBackoffMinutes, maxRideRetries)
	if err != nil {
		slog.Warn("failed to fetch unmatched rides for retry", "error", err)
		return
	}

	for _, ride := range rides {
		if ride.RetryCount >= maxRideRetries {
			s.escalateUnmatchedRide(ctx, &ride)
			continue
		}

		slog.Info("retrying unmatched ride", "ride_id", ride.RideID, "retry", ride.RetryCount+1)

		// Re-broadcast via RequestRide (deduplication prevents duplicate offers)
		_, err := s.RequestRide(ctx, &ride)
		if err != nil {
			slog.Warn("retry: re-broadcast failed", "ride_id", ride.RideID, "error", err)
		}

		// Increment retry count
		if err := s.repo.IncrementRetry(ctx, ride.RideID); err != nil {
			slog.Warn("retry: failed to increment retry count", "ride_id", ride.RideID, "error", err)
		}
	}
}

// escalateUnmatchedRide marks a ride as unmatched and logs for admin alerting.
func (s *RideService) escalateUnmatchedRide(ctx context.Context, ride *model.Ride) {
	slog.Error("ESCALATION: ride unmatched after max retries",
		"ride_id", ride.RideID,
		"retry_count", ride.RetryCount,
		"booking_id", ride.BookingID,
		"pickup_address", ride.PickupAddress,
	)

	// Mark as unmatched
	if err := s.repo.UpdateStatus(ctx, ride.RideID, "unmatched"); err != nil {
		slog.Error("escalate: failed to set unmatched status", "ride_id", ride.RideID, "error", err)
	}
}

func (s *RideService) GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error) {
	return s.repo.GetRideByBookingID(ctx, bookingID)
}

func (s *RideService) GetRidesByBookingID(ctx context.Context, bookingID int64) ([]model.Ride, error) {
	return s.repo.GetRidesByBookingID(ctx, bookingID)
}

func (s *RideService) CancelRide(ctx context.Context, rideID int64) error {
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}
	if err := s.repo.CancelRide(ctx, rideID); err != nil {
		return err
	}

	// Explicitly expire pending ride offers when a ride is cancelled so riders
	// get immediate invalidation instead of waiting for TTL expiry.
	if s.offerRepo != nil {
		expiredOffers, err := s.offerRepo.ExpireOffersForRide(ctx, rideID)
		if err != nil {
			slog.Warn("CancelRide: failed to expire ride offers", "ride_id", rideID, "error", err)
		} else {
			for _, offer := range expiredOffers {
				_ = broadcaster.BroadcastToUser(offer.RiderID, "ride_offer", map[string]any{
					"offer_id": offer.OfferID,
					"ride_id":  offer.RideID,
					"status":   model.RideOfferStatusExpired,
					"reason":   "ride_cancelled",
				})
			}
		}
	}

	// Notify rider if assigned
	if ride.RiderID != nil {
		_ = broadcaster.BroadcastToUser(*ride.RiderID, "ride:cancelled", map[string]any{
			"ride_id": rideID,
		})
	}
	// Notify passenger
	_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:cancelled", map[string]any{
		"ride_id": rideID,
	})
	return nil
}

func (s *RideService) GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error) {
	return s.repo.GetProfileByRiderID(ctx, riderID)
}

func (s *RideService) UnassignRider(ctx context.Context, rideID int64) error {
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}

	if ride.RiderID == nil {
		return nil // Already unassigned
	}

	oldRiderID := *ride.RiderID

	if err := s.repo.UnassignRider(ctx, rideID); err != nil {
		return err
	}

	// Notify previous rider
	_ = broadcaster.BroadcastToUser(oldRiderID, "ride:unassigned", map[string]any{
		"ride_id": rideID,
	})

	// Notify passenger/client
	_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:updated", map[string]any{
		"ride_id": rideID,
		"status":  "pending",
	})

	return nil
}

func (s *RideService) ForceAssignRider(ctx context.Context, rideID, riderID int64) error {
	// 1. Verify ride
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}

	// 2. Assign in repo (sets status to 'offered' internally)
	if err := s.repo.AssignRider(ctx, rideID, riderID); err != nil {
		return err
	}

	// 3. Force to 'accepted' status
	if err := s.repo.UpdateStatus(ctx, rideID, "accepted"); err != nil {
		return err
	}

	// 4. Notify new rider
	_ = broadcaster.BroadcastToUser(riderID, "ride:assigned", map[string]any{
		"ride_id": rideID,
	})

	// 5. Notify passenger/client
	_ = broadcaster.BroadcastToUser(ride.PassengerID, "ride:updated", map[string]any{
		"ride_id":  rideID,
		"status":   "accepted",
		"rider_id": riderID,
	})

	return nil
}
