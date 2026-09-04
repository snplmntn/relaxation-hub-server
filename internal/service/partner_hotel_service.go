package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type PartnerHotelService struct {
	repo repository.PartnerHotelRepository
}

func NewPartnerHotelService(repo repository.PartnerHotelRepository) *PartnerHotelService {
	return &PartnerHotelService{repo: repo}
}

func (s *PartnerHotelService) CreateHotel(ctx context.Context, req *model.CreatePartnerHotelRequest) (*model.PartnerHotel, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	hotel := &model.PartnerHotel{
		HotelName:     strings.TrimSpace(req.HotelName),
		AddressLine:   strings.TrimSpace(req.AddressLine),
		City:          strings.TrimSpace(req.City),
		ContactPerson: strings.TrimSpace(req.ContactPerson),
		Email:         strings.TrimSpace(req.Email),
		Phone:         strings.TrimSpace(req.Phone),
		Notes:         strings.TrimSpace(req.Notes),
		IsActive:      true,
	}
	if err := validatePartnerHotel(hotel); err != nil {
		return nil, err
	}
	if err := s.repo.CreateHotel(ctx, hotel); err != nil {
		return nil, err
	}
	return hotel, nil
}

func (s *PartnerHotelService) ListHotels(ctx context.Context) ([]model.PartnerHotel, error) {
	return s.repo.ListHotels(ctx)
}

func (s *PartnerHotelService) UpdateHotel(ctx context.Context, hotelID int64, req *model.UpdatePartnerHotelRequest) (*model.PartnerHotel, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	hotel, err := s.repo.GetHotel(ctx, hotelID)
	if err != nil {
		return nil, err
	}
	if req.HotelName != nil {
		hotel.HotelName = strings.TrimSpace(*req.HotelName)
	}
	if req.AddressLine != nil {
		hotel.AddressLine = strings.TrimSpace(*req.AddressLine)
	}
	if req.City != nil {
		hotel.City = strings.TrimSpace(*req.City)
	}
	if req.ContactPerson != nil {
		hotel.ContactPerson = strings.TrimSpace(*req.ContactPerson)
	}
	if req.Email != nil {
		hotel.Email = strings.TrimSpace(*req.Email)
	}
	if req.Phone != nil {
		hotel.Phone = strings.TrimSpace(*req.Phone)
	}
	if req.Notes != nil {
		hotel.Notes = strings.TrimSpace(*req.Notes)
	}
	if req.IsActive != nil {
		hotel.IsActive = *req.IsActive
	}
	if err := validatePartnerHotel(hotel); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateHotel(ctx, hotel); err != nil {
		return nil, err
	}
	return s.repo.GetHotel(ctx, hotelID)
}

func (s *PartnerHotelService) CreateStaff(ctx context.Context, hotelID int64, req *model.CreatePartnerHotelStaffRequest) (*model.PartnerHotelStaff, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	hotel, err := s.repo.GetHotel(ctx, hotelID)
	if err != nil {
		return nil, err
	}
	if !hotel.IsActive {
		return nil, fmt.Errorf("cannot add staff to an inactive partnered hotel")
	}
	staff := &model.PartnerHotelStaff{
		PartnerHotelID: hotelID,
		FullName:       strings.TrimSpace(req.FullName),
		Position:       strings.TrimSpace(req.Position),
		Email:          strings.TrimSpace(req.Email),
		Phone:          strings.TrimSpace(req.Phone),
		IsActive:       true,
	}
	if err := validatePartnerHotelStaff(staff); err != nil {
		return nil, err
	}
	if err := s.repo.CreateStaff(ctx, staff); err != nil {
		return nil, err
	}
	return staff, nil
}

func (s *PartnerHotelService) ListStaff(ctx context.Context, hotelID int64) ([]model.PartnerHotelStaff, error) {
	if _, err := s.repo.GetHotel(ctx, hotelID); err != nil {
		return nil, err
	}
	return s.repo.ListStaff(ctx, hotelID)
}

func (s *PartnerHotelService) UpdateStaff(ctx context.Context, hotelID, staffID int64, req *model.UpdatePartnerHotelStaffRequest) (*model.PartnerHotelStaff, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	staff, err := s.repo.GetStaff(ctx, hotelID, staffID)
	if err != nil {
		return nil, err
	}
	if req.FullName != nil {
		staff.FullName = strings.TrimSpace(*req.FullName)
	}
	if req.Position != nil {
		staff.Position = strings.TrimSpace(*req.Position)
	}
	if req.Email != nil {
		staff.Email = strings.TrimSpace(*req.Email)
	}
	if req.Phone != nil {
		staff.Phone = strings.TrimSpace(*req.Phone)
	}
	if req.IsActive != nil {
		staff.IsActive = *req.IsActive
	}
	if err := validatePartnerHotelStaff(staff); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStaff(ctx, staff); err != nil {
		return nil, err
	}
	return s.repo.GetStaff(ctx, hotelID, staffID)
}

func validatePartnerHotel(hotel *model.PartnerHotel) error {
	if hotel.HotelName == "" {
		return fmt.Errorf("hotel_name is required")
	}
	if len(hotel.HotelName) > 160 {
		return fmt.Errorf("hotel_name must be 160 characters or fewer")
	}
	if err := validateOptionalEmail(hotel.Email); err != nil {
		return err
	}
	return nil
}

func validatePartnerHotelStaff(staff *model.PartnerHotelStaff) error {
	if staff.FullName == "" {
		return fmt.Errorf("full_name is required")
	}
	if len(staff.FullName) > 160 {
		return fmt.Errorf("full_name must be 160 characters or fewer")
	}
	return validateOptionalEmail(staff.Email)
}

func validateOptionalEmail(value string) error {
	if value == "" {
		return nil
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return fmt.Errorf("email must be a valid email address")
	}
	return nil
}
