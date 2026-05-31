package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type AddressService struct {
	repo     repository.AddressRepository
	geocoder Geocoder
}

func NewAddressService(repo repository.AddressRepository, geocoder Geocoder) *AddressService {
	return &AddressService{
		repo:     repo,
		geocoder: geocoder,
	}
}

func (s *AddressService) SetGeocoder(g Geocoder) {
	s.geocoder = g
}

func (s *AddressService) Create(ctx context.Context, userID int64, req *model.CreateAddressRequest) (*model.Address, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	trim := func(v string) string { return strings.TrimSpace(v) }
	req.Street = trim(req.Street)
	req.City = trim(req.City)
	req.Label = trim(req.Label)
	req.Province = trim(req.Province)
	req.PostalCode = trim(req.PostalCode)
	req.Barangay = trim(req.Barangay)
	req.Landmark = trim(req.Landmark)
	if req.Country == "" {
		req.Country = "Philippines"
	} else {
		req.Country = trim(req.Country)
	}

	if req.Street == "" || req.City == "" {
		return nil, fmt.Errorf("street and city are required")
	}

	var lat *float64
	var lon *float64

	// Debug: Check if coordinates are provided
	println("DEBUG: req.Latitude =", req.Latitude, "req.Longitude =", req.Longitude)

	// Use client-provided coordinates if available
	if req.Latitude != nil && req.Longitude != nil {
		println("DEBUG: Using client-provided coordinates")
		// Validate coordinates are within Philippines
		if !isLatLonWithinPH(*req.Latitude, *req.Longitude) {
			return nil, fmt.Errorf("coordinates out of supported range")
		}
		lat = req.Latitude
		lon = req.Longitude
	} else if s.geocoder != nil {
		println("DEBUG: Geocoding address because coordinates not provided")
		// Fall back to geocoding if coordinates not provided
		fullAddress := strings.Join([]string{
			req.Street, req.Barangay, req.City, req.Province, req.PostalCode, req.Country,
		}, ", ")

		result, err := s.geocoder.Geocode(ctx, fullAddress)
		if err != nil {
			return nil, err
		}
		lat = &result.Latitude
		lon = &result.Longitude
		if !isLatLonWithinPH(*lat, *lon) {
			return nil, fmt.Errorf("coordinates out of supported range")
		}
	}
	// If no coordinates and no geocoder, allow creation without coordinates

	addr := &model.Address{
		UserID:     userID,
		Label:      req.Label,
		Street:     req.Street,
		Barangay:   req.Barangay,
		City:       req.City,
		Province:   req.Province,
		PostalCode: req.PostalCode,
		Landmark:   req.Landmark,
		Country:    req.Country,
		Latitude:   lat,
		Longitude:  lon,
		IsDefault:  req.IsDefault,
	}

	if err := s.repo.Create(ctx, addr); err != nil {
		return nil, err
	}

	return addr, nil
}

func (s *AddressService) GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error) {
	addr, err := s.repo.GetByID(ctx, addressID, userID)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func (s *AddressService) List(ctx context.Context, userID int64) ([]model.Address, error) {
	return s.repo.ListForUser(ctx, userID, false)
}

func (s *AddressService) Update(ctx context.Context, addressID, userID int64, req *model.UpdateAddressRequest) (*model.Address, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	addr, err := s.repo.GetByID(ctx, addressID, userID)
	if err != nil {
		return nil, err
	}

	// Track whether we need to re-geocode
	needsGeocode := false

	if req.Label != nil {
		addr.Label = strings.TrimSpace(*req.Label)
	}
	if req.Street != nil {
		addr.Street = strings.TrimSpace(*req.Street)
		needsGeocode = true
	}
	if req.Barangay != nil {
		addr.Barangay = strings.TrimSpace(*req.Barangay)
		needsGeocode = true
	}
	if req.City != nil {
		addr.City = strings.TrimSpace(*req.City)
		needsGeocode = true
	}
	if req.Province != nil {
		addr.Province = strings.TrimSpace(*req.Province)
		needsGeocode = true
	}
	if req.PostalCode != nil {
		addr.PostalCode = strings.TrimSpace(*req.PostalCode)
		needsGeocode = true
	}
	if req.Landmark != nil {
		addr.Landmark = strings.TrimSpace(*req.Landmark)
	}
	if req.Country != nil {
		addr.Country = strings.TrimSpace(*req.Country)
		needsGeocode = true
	}

	coordsProvided := req.Latitude != nil || req.Longitude != nil
	if coordsProvided {
		if req.Latitude == nil || req.Longitude == nil {
			return nil, fmt.Errorf("latitude and longitude must be provided together")
		}
		if !isLatLonWithinPH(*req.Latitude, *req.Longitude) {
			return nil, fmt.Errorf("coordinates out of supported range")
		}
		addr.Latitude = req.Latitude
		addr.Longitude = req.Longitude
		needsGeocode = false
	}

	if needsGeocode {
		if s.geocoder == nil {
			// Without geocoder, keep existing coordinates (if any) and proceed.
		} else {
			fullAddress := strings.Join([]string{
				addr.Street, addr.Barangay, addr.City, addr.Province, addr.PostalCode, addr.Country,
			}, ", ")
			result, err := s.geocoder.Geocode(ctx, fullAddress)
			if err != nil {
				return nil, err
			}
			addr.Latitude = &result.Latitude
			addr.Longitude = &result.Longitude
			if !isLatLonWithinPH(*addr.Latitude, *addr.Longitude) {
				return nil, fmt.Errorf("coordinates out of supported range")
			}
		}
	}

	if err := s.repo.Update(ctx, addr); err != nil {
		return nil, err
	}
	if req.IsDefault != nil && *req.IsDefault {
		if err := s.repo.SetDefault(ctx, addressID, userID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, addressID, userID)
}

func (s *AddressService) SetDefault(ctx context.Context, addressID, userID int64) error {
	if _, err := s.repo.GetByID(ctx, addressID, userID); err != nil {
		return err
	}
	return s.repo.SetDefault(ctx, addressID, userID)
}

func (s *AddressService) Delete(ctx context.Context, addressID, userID int64) error {
	if _, err := s.repo.GetByID(ctx, addressID, userID); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, addressID, userID)
}

// SetDisabled disables or re-enables an address (reversible alternative to delete).
func (s *AddressService) SetDisabled(ctx context.Context, addressID, userID int64, disabled bool) error {
	if _, err := s.repo.GetByID(ctx, addressID, userID); err != nil {
		return err
	}
	return s.repo.SetDisabled(ctx, addressID, userID, disabled)
}

func isLatLonWithinPH(lat, lon float64) bool {
	if lat < 5 || lat > 20 {
		return false
	}
	return lon >= 116 && lon <= 127
}
