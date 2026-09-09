package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
	"strings"
	"unicode/utf8"

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
		AccessRole:     req.AccessRole,
		PartnerHotelID: hotelID,
		FullName:       strings.TrimSpace(req.FullName),
		Position:       strings.TrimSpace(req.Position),
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		Phone:          strings.TrimSpace(req.Phone),
		IsActive:       true,
	}
	if err := validatePartnerHotelStaff(staff); err != nil {
		return nil, err
	}
	if staff.AccessRole == "" {
		staff.AccessRole = model.RoleHotelStaff
	}
	if err := prepareHotelCredentials(staff, req.Password, true); err != nil {
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
		staff.Email = strings.ToLower(strings.TrimSpace(*req.Email))
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
	if req.AccessRole != nil {
		staff.AccessRole = *req.AccessRole
	}
	password := ""
	if req.Password != nil {
		password = *req.Password
	}
	if err := prepareHotelCredentials(staff, password, false); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStaff(ctx, staff); err != nil {
		return nil, err
	}
	return s.repo.GetStaff(ctx, hotelID, staffID)
}

func prepareHotelCredentials(staff *model.PartnerHotelStaff, password string, creating bool) error {
	if !model.IsHotelRole(staff.AccessRole) {
		return fmt.Errorf("access_role must be hotel_admin or hotel_staff")
	}
	if creating || staff.UserID != nil || password != "" {
		if staff.Email == "" || !isEmailValid(staff.Email) || len(staff.Email) > 100 {
			return fmt.Errorf("a valid login email of 100 characters or fewer is required")
		}
		if len(staff.FullName) > 100 {
			return fmt.Errorf("full_name must be 100 characters or fewer for a login account")
		}
		if len(staff.Phone) > 20 {
			return fmt.Errorf("phone must be 20 characters or fewer")
		}
	}
	if creating && password == "" {
		return fmt.Errorf("password is required for Hiraya access")
	}
	if password != "" {
		if err := validatePassword(password); err != nil {
			return err
		}
		if len(password) > 72 {
			return fmt.Errorf("password must be 72 bytes or fewer")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		staff.PasswordHash = string(hash)
	}
	return nil
}

var ErrHotelAccessDenied = errors.New("active hotel staff access is required")

func (s *PartnerHotelService) GetAccess(ctx context.Context, userID int64) (*model.HotelAccess, error) {
	access, err := s.repo.GetHotelAccess(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrHotelAccessDenied
	}
	return access, err
}

func (s *PartnerHotelService) ListMyHotelStaff(ctx context.Context, userID int64) ([]model.PartnerHotelStaff, error) {
	access, err := s.GetAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	if access.AccessRole != model.RoleHotelAdmin {
		return nil, ErrHotelAccessDenied
	}
	return s.repo.ListStaff(ctx, access.PartnerHotelID)
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
	for _, field := range []struct {
		name, value string
		max         int
	}{
		{"address_line", hotel.AddressLine, 255}, {"city", hotel.City, 120}, {"contact_person", hotel.ContactPerson, 160}, {"email", hotel.Email, 255}, {"phone", hotel.Phone, 40},
	} {
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s must be %d characters or fewer", field.name, field.max)
		}
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
	for _, field := range []struct {
		name, value string
		max         int
	}{
		{"position", staff.Position, 120}, {"email", staff.Email, 255}, {"phone", staff.Phone, 40},
	} {
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s must be %d characters or fewer", field.name, field.max)
		}
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
