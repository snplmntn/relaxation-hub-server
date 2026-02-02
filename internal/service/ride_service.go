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

	// 3. Find Match (Async or Synchronous)
	// For immediate feedback, let's try to find a rider now.
    // Radius 5km
	riders, err := s.matchingService.FindNearbyRiders(ctx, ride.PickupLat, ride.PickupLong, 5.0)
	if err != nil {
        // Log error but return created ride
		return ride, nil 
	}
    
    if len(riders) > 0 {
        // Assign to first rider for MVP (Round Robin / Greedy)
        // In SOTA, we'd notify all or use a scoring system.
        bestRider := riders[0]
        if err := s.repo.AssignRider(ctx, ride.RideID, bestRider.RiderID); err != nil {
             return ride, nil // Return pending
        }
        // Update local object
        ride.RiderID = &bestRider.RiderID
        ride.Status = "offered"
        
        // Send notifications to rider
        if s.notificationSvc != nil {
        	// Send FCM push notification
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
        	
        	s.notificationSvc.SendPushDirect(ctx, bestRider.UserID, "ride_offer", title, message, data)
        	
        	// Broadcast via WebSocket
        	_ = broadcaster.BroadcastToUser(bestRider.UserID, "ride_offer", ride)
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
    return s.repo.GetRidesForRiderByStatus(ctx, riderID, "offered")
}

func (s *RideService) AcceptRide(ctx context.Context, rideID, riderID int64) error {
	// Verify ride is assigned to this rider and is 'offered'
	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}
	if ride.RiderID == nil || *ride.RiderID != riderID {
		return errors.New("ride not assigned to you")
	}
	if ride.Status != "offered" {
		return errors.New("ride no longer available")
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
