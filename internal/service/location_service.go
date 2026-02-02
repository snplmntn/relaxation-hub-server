package service

import (
	"context"
	"errors"
	"math"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// LocationService handles location validation and geofencing logic.
type LocationService struct {
	repo repository.ServiceAreaRepository
}

// NewLocationService creates a new LocationService.
func NewLocationService(repo repository.ServiceAreaRepository) *LocationService {
	return &LocationService{repo: repo}
}

// CheckLocationStatus validates if a location is serviceable.
// Returns the status and an appropriate user-facing message.
// If the area is not supported, records the user's interest automatically.
func (s *LocationService) CheckLocationStatus(ctx context.Context, userID int64, cityCode, barangayCode string) (*model.LocationCheckResult, error) {
	result := &model.LocationCheckResult{
		Status:    model.ServiceAreaStatusNotSupported,
		Message:   "We don't serve this area yet. We've noted your interest!",
		IsAllowed: false,
	}

	// Step 1: Check barangay first (more specific)
	if barangayCode != "" {
		barangayStatus, err := s.repo.GetStatusByCode(ctx, barangayCode)
		if err != nil {
			return nil, err
		}

		// If barangay is explicitly banned, reject immediately
		if barangayStatus == model.ServiceAreaStatusBanned {
			result.Status = model.ServiceAreaStatusBanned
			// Same message to avoid offending user
			result.Message = "We don't serve this area yet. We've noted your interest!"
			result.IsAllowed = false

			// Still record interest (we might expand safely later)
			if userID > 0 {
				_ = s.repo.RecordInterest(ctx, userID, barangayCode)
			}
			return result, nil
		}

		// If barangay is explicitly covered, allow
		if barangayStatus == model.ServiceAreaStatusCovered {
			area, err := s.repo.GetByCode(ctx, barangayCode)
			if err == nil {
				result.Status = model.ServiceAreaStatusCovered
				result.Message = ""
				result.IsAllowed = true
				result.AreaName = area.Name
				result.MinBooking = area.MinBookingMinutes
				return result, nil
			}
		}
	}

	// Step 2: Check city level
	cityStatus, err := s.repo.GetStatusByCode(ctx, cityCode)
	if err != nil {
		return nil, err
	}

	switch cityStatus {
	case model.ServiceAreaStatusCovered:
		area, err := s.repo.GetByCode(ctx, cityCode)
		if err == nil {
			result.Status = model.ServiceAreaStatusCovered
			result.Message = ""
			result.IsAllowed = true
			result.AreaName = area.Name
			result.MinBooking = area.MinBookingMinutes
		}

	case model.ServiceAreaStatusBanned:
		result.Status = model.ServiceAreaStatusBanned
		result.Message = "We don't serve this area yet. We've noted your interest!"
		result.IsAllowed = false
		if userID > 0 {
			_ = s.repo.RecordInterest(ctx, userID, cityCode)
		}

	default: // not_supported
		result.Status = model.ServiceAreaStatusNotSupported
		result.Message = "We don't serve this area yet. We've noted your interest!"
		result.IsAllowed = false
		if userID > 0 {
			_ = s.repo.RecordInterest(ctx, userID, cityCode)
		}
	}

	return result, nil
}

// CheckLocationByName validates if a location is serviceable using city/barangay names.
// This is the primary method used by the map picker which provides names from reverse geocoding.
// Returns the status and an appropriate user-facing message.
// If the area is not supported, records the user's interest automatically.
func (s *LocationService) CheckLocationByName(ctx context.Context, userID int64, cityName, barangayName string) (*model.LocationCheckResult, error) {
	result := &model.LocationCheckResult{
		Status:    model.ServiceAreaStatusNotSupported,
		Message:   "We don't serve this area yet. We've noted your interest!",
		IsAllowed: false,
	}

	// Step 1: Try to find barangay by name first (more specific)
	if barangayName != "" {
		area, err := s.repo.GetByName(ctx, barangayName, model.ServiceAreaLevelBarangay)
		if err == nil && area != nil {
			switch area.Status {
			case model.ServiceAreaStatusCovered:
				result.Status = model.ServiceAreaStatusCovered
				result.Message = ""
				result.IsAllowed = true
				result.AreaName = area.Name
				result.MinBooking = area.MinBookingMinutes
				return result, nil
			case model.ServiceAreaStatusBanned:
				result.Status = model.ServiceAreaStatusBanned
				result.Message = "We don't serve this area yet. We've noted your interest!"
				result.IsAllowed = false
				if userID > 0 {
					_ = s.repo.RecordInterest(ctx, userID, area.PSGCCode)
				}
				return result, nil
			}
			// If not_supported, fall through to city check
		}
	}

	// Step 2: Check city level by name
	if cityName != "" {
		area, err := s.repo.GetByName(ctx, cityName, model.ServiceAreaLevelCity)
		if err == nil && area != nil {
			switch area.Status {
			case model.ServiceAreaStatusCovered:
				result.Status = model.ServiceAreaStatusCovered
				result.Message = ""
				result.IsAllowed = true
				result.AreaName = area.Name
				result.MinBooking = area.MinBookingMinutes
				return result, nil
			case model.ServiceAreaStatusBanned:
				result.Status = model.ServiceAreaStatusBanned
				result.Message = "We don't serve this area yet. We've noted your interest!"
				result.IsAllowed = false
				if userID > 0 {
					_ = s.repo.RecordInterest(ctx, userID, area.PSGCCode)
				}
				return result, nil
			default:
				// not_supported - record interest
				if userID > 0 {
					_ = s.repo.RecordInterest(ctx, userID, area.PSGCCode)
				}
			}
		}
	}

	// Area not found or not supported
	return result, nil
}

// CreateServiceArea creates or updates a service area.
func (s *LocationService) CreateServiceArea(ctx context.Context, area *model.ServiceArea) error {
	// Basic validation
	if area.PSGCCode == "" || area.Name == "" {
		return errors.New("psgc_code and name are required")
	}
	// Default status if empty
	if area.Status == "" {
		area.Status = model.ServiceAreaStatusNotSupported
	}
	return s.repo.UpsertArea(ctx, area)
}

// GetDistance calculates the Haversine distance between two coordinates in meters.
// This is used for distance-based booking rules without external API calls.
func (s *LocationService) GetDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000 // Earth's radius in meters

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	// Haversine formula
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusM * c
}

// GetDistanceKm is a convenience wrapper that returns distance in kilometers.
func (s *LocationService) GetDistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	return s.GetDistance(lat1, lng1, lat2, lng2) / 1000
}

// GetCoveredAreas returns all areas that are currently serviceable.
func (s *LocationService) GetCoveredAreas(ctx context.Context) ([]model.ServiceArea, error) {
	return s.repo.ListByStatus(ctx, model.ServiceAreaStatusCovered)
}

// GetTopDemandAreas returns areas with the most user interest (for expansion planning).
func (s *LocationService) GetTopDemandAreas(ctx context.Context, limit int) ([]model.ServiceArea, error) {
	return s.repo.ListTopDemand(ctx, limit)
}

// GetInterestedUsers returns all users who requested coverage for an area.
// Used for re-engagement campaigns when launching in a new area.
func (s *LocationService) GetInterestedUsers(ctx context.Context, psgcCode string) ([]int64, error) {
	return s.repo.ListInterestedUsers(ctx, psgcCode)
}

// SetAreaStatus updates the operational status of an area (admin function).
func (s *LocationService) SetAreaStatus(ctx context.Context, psgcCode string, status model.ServiceAreaStatus) error {
	return s.repo.UpdateStatus(ctx, psgcCode, status)
}
