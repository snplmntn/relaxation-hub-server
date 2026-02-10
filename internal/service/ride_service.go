package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrNoRidersAvailable = errors.New("no riders available nearby")
)

type RideService struct {
	repo            repository.RideRepository
	pricingService  *RidePricingService
	matchingService *RideMatchingService
	notificationSvc *NotificationService
	geocoder        Geocoder
	db              db.DBTX
}

func NewRideService(repo repository.RideRepository, pricing *RidePricingService, matching *RideMatchingService, db db.DBTX) *RideService {
	return &RideService{
		repo:            repo,
		pricingService:  pricing,
		matchingService: matching,
		notificationSvc: nil,
		geocoder:        nil,
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
	// Radius 5km
	riders, err := s.matchingService.FindNearbyRiders(ctx, ride.PickupLat, ride.PickupLong, 5.0)
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
        	
			// Send to each rider
			for _, r := range riders {
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
    // Return rides assigned to rider (offered) OR pending rides nearby?
	// For "Broadcast" model, a rider sees "Available" rides. 
	// This method might be deprecated or used for "Direct Offers".
	// For now, let's keep it as is for backward compat, but we might want GetAvailableRides
    return s.repo.GetRidesForRiderByStatus(ctx, riderID, "offered")
}

// GetAvailableRides returns rides that are pending and near the rider
func (s *RideService) GetAvailableRides(ctx context.Context, riderID int64, lat, long, radiusKm float64) ([]model.Ride, error) {
	return s.repo.GetAvailableRidesNear(ctx, lat, long, radiusKm)
}

func (s *RideService) AcceptRide(ctx context.Context, rideID, riderID int64) error {
	// 1. Verify ride exists
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}
	
	// 2. Race Condition Check: Is it still pending?
	if ride.Status != "pending" {
		// If it's already offered to ME (Direct Assign backup), I can accept.
		if ride.Status == "offered" && ride.RiderID != nil && *ride.RiderID == riderID {
			// Allow verify
		} else {
			return errors.New("ride no longer available")
		}
	}

	// 3. Assign to Rider using explicit Update to avoid race
	// We use the Repo's AssignRider but we need to change status to 'accepted' immediately or 'offered' then 'accepted'?
	// The standard flow is -> 'accepted'.
	// But AssignRider sets it to 'offered'. 
	// Let's call AssignRider then UpdateStatus, or create a new ClaimRide method.
	// For now, AssignRider sets 'offered'. Then we set 'accepted'.
	
	if err := s.repo.AssignRider(ctx, rideID, riderID); err != nil {
		return err
	}
	
	return s.repo.UpdateStatus(ctx, rideID, "accepted")
}

func (s *RideService) UpdateRideStatus(ctx context.Context, rideID, riderID int64, status string) error {
    // Basic validation
    return s.repo.UpdateStatus(ctx, rideID, status)
}

func (s *RideService) UpdateRiderLocation(ctx context.Context, riderID int64, lat, long float64) error {
    return s.repo.UpdateRiderLocation(ctx, riderID, lat, long)
}

func (s *RideService) UpdateRiderLocationByUserID(ctx context.Context, userID int64, lat, long float64) error {
	rider, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.UpdateRiderLocation(ctx, rider.RiderID, lat, long)
}

func (s *RideService) ToggleOnlineStatus(ctx context.Context, userID int64, isOnline bool) error {
    profile, err := s.repo.GetRiderProfile(ctx, userID)
    if err != nil {
        return err
    }
    return s.repo.UpdateRiderStatus(ctx, profile.RiderID, isOnline)
}

func (s *RideService) GetRiderOffersByUserID(ctx context.Context, userID int64) ([]model.Ride, error) {
	profile, err := s.repo.GetRiderProfile(ctx, userID)
	if err != nil {
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
		return nil, err
	}
	// Query repo for active ride
	return s.repo.GetActiveRideByRiderID(ctx, profile.RiderID)
}

func (s *RideService) DeclineRide(ctx context.Context, rideID, userID int64) error {
	// For broadcast system, declining is mostly client-side.
	// We could track it in a 'ride_declines' table for analytics or re-dispatch logic.
	// For now, we just acknowledge it.
	return nil
}

func (s *RideService) GetRideByBookingID(ctx context.Context, bookingID int64) (*model.Ride, error) {
	return s.repo.GetRideByBookingID(ctx, bookingID)
}

func (s *RideService) GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error) {
	return s.repo.GetProfileByRiderID(ctx, riderID)
}
