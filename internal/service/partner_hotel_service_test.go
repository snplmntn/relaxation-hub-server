package service

import (
	"context"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type partnerHotelServiceRepo struct {
	access        *model.HotelAccess
	listedHotelID int64
	hotel         *model.PartnerHotel
	createdStaff  *model.PartnerHotelStaff
}

func TestPartnerHotelRejectsOversizeFieldsBeforePersistence(t *testing.T) {
	repo := &partnerHotelServiceRepo{hotel: &model.PartnerHotel{IsActive: true}}
	svc := NewPartnerHotelService(repo)
	_, err := svc.CreateHotel(context.Background(), &model.CreatePartnerHotelRequest{HotelName: "Hotel", City: strings.Repeat("x", 121)})
	require.ErrorContains(t, err, "city must be 120 characters or fewer")
	_, err = svc.CreateStaff(context.Background(), 1, &model.CreatePartnerHotelStaffRequest{FullName: "Staff", Position: strings.Repeat("x", 121), Email: "valid@example.com", Password: "Password123!"})
	require.ErrorContains(t, err, "position must be 120 characters or fewer")
	require.Nil(t, repo.createdStaff)
}

func (r *partnerHotelServiceRepo) GetHotelAccess(_ context.Context, _ int64) (*model.HotelAccess, error) {
	if r.access == nil {
		return nil, pgx.ErrNoRows
	}
	return r.access, nil
}

func (r *partnerHotelServiceRepo) CreateHotel(_ context.Context, hotel *model.PartnerHotel) error {
	hotel.PartnerHotelID = 10
	r.hotel = hotel
	return nil
}

func (r *partnerHotelServiceRepo) GetHotel(_ context.Context, _ int64) (*model.PartnerHotel, error) {
	return r.hotel, nil
}

func (r *partnerHotelServiceRepo) ListHotels(_ context.Context) ([]model.PartnerHotel, error) {
	return nil, nil
}

func (r *partnerHotelServiceRepo) UpdateHotel(_ context.Context, hotel *model.PartnerHotel) error {
	r.hotel = hotel
	return nil
}

func (r *partnerHotelServiceRepo) CreateStaff(_ context.Context, staff *model.PartnerHotelStaff) error {
	staff.PartnerHotelStaffID = 20
	r.createdStaff = staff
	return nil
}

func (r *partnerHotelServiceRepo) GetStaff(_ context.Context, _, _ int64) (*model.PartnerHotelStaff, error) {
	return r.createdStaff, nil
}

func (r *partnerHotelServiceRepo) ListStaff(_ context.Context, hotelID int64) ([]model.PartnerHotelStaff, error) {
	r.listedHotelID = hotelID
	return nil, nil
}

func TestHotelAccountCredentials(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff} {
		t.Run(role, func(t *testing.T) {
			repo := &partnerHotelServiceRepo{hotel: &model.PartnerHotel{IsActive: true}}
			staff, err := NewPartnerHotelService(repo).CreateStaff(context.Background(), 10, &model.CreatePartnerHotelStaffRequest{
				FullName: "  Maria Santos  ", Email: "  MARIA@example.com  ", AccessRole: role, Password: "Password123!",
			})
			require.NoError(t, err)
			assert.Equal(t, "maria@example.com", staff.Email)
			assert.Equal(t, role, staff.AccessRole)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte("Password123!")))
		})
	}
	for _, test := range []struct{ name, role, password, email string }{
		{"platform role", model.RoleSuperAdmin, "Password123!", "staff@example.com"},
		{"missing password", model.RoleHotelStaff, "", "staff@example.com"},
		{"weak password", model.RoleHotelStaff, "password", "staff@example.com"},
		{"missing email", model.RoleHotelStaff, "Password123!", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &partnerHotelServiceRepo{hotel: &model.PartnerHotel{IsActive: true}}
			_, err := NewPartnerHotelService(repo).CreateStaff(context.Background(), 10, &model.CreatePartnerHotelStaffRequest{FullName: "Staff", Email: test.email, AccessRole: test.role, Password: test.password})
			require.Error(t, err)
			assert.Nil(t, repo.createdStaff)
		})
	}
}

func TestHotelDirectoryAuthorization(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff, ""} {
		t.Run(role, func(t *testing.T) {
			repo := &partnerHotelServiceRepo{}
			if role != "" {
				repo.access = &model.HotelAccess{PartnerHotelID: 42, AccessRole: role}
			}
			_, err := NewPartnerHotelService(repo).ListMyHotelStaff(context.Background(), 7)
			if role == model.RoleHotelAdmin {
				require.NoError(t, err)
				assert.Equal(t, int64(42), repo.listedHotelID)
			} else {
				require.ErrorIs(t, err, ErrHotelAccessDenied)
				assert.Zero(t, repo.listedHotelID)
			}
		})
	}
}

func (r *partnerHotelServiceRepo) UpdateStaff(_ context.Context, staff *model.PartnerHotelStaff) error {
	r.createdStaff = staff
	return nil
}

func TestPartnerHotelServiceCreateHotelNormalizesInput(t *testing.T) {
	repo := &partnerHotelServiceRepo{}
	svc := NewPartnerHotelService(repo)

	hotel, err := svc.CreateHotel(context.Background(), &model.CreatePartnerHotelRequest{
		HotelName: "  Bayview Hotel  ",
		City:      "  Manila  ",
		Email:     "partner@example.com",
	})

	require.NoError(t, err)
	assert.Equal(t, "Bayview Hotel", hotel.HotelName)
	assert.Equal(t, "Manila", hotel.City)
	assert.True(t, hotel.IsActive)
	assert.Equal(t, int64(10), hotel.PartnerHotelID)
}

func TestPartnerHotelServiceCreateStaffRejectsInactiveHotel(t *testing.T) {
	repo := &partnerHotelServiceRepo{hotel: &model.PartnerHotel{
		PartnerHotelID: 10,
		HotelName:      "Bayview Hotel",
		IsActive:       false,
	}}
	svc := NewPartnerHotelService(repo)

	staff, err := svc.CreateStaff(context.Background(), 10, &model.CreatePartnerHotelStaffRequest{
		FullName: "Maria Santos",
	})

	assert.Nil(t, staff)
	assert.EqualError(t, err, "cannot add staff to an inactive partnered hotel")
	assert.Nil(t, repo.createdStaff)
}

func TestPartnerHotelServiceRejectsInvalidStaffEmail(t *testing.T) {
	repo := &partnerHotelServiceRepo{hotel: &model.PartnerHotel{
		PartnerHotelID: 10,
		HotelName:      "Bayview Hotel",
		IsActive:       true,
	}}
	svc := NewPartnerHotelService(repo)

	staff, err := svc.CreateStaff(context.Background(), 10, &model.CreatePartnerHotelStaffRequest{
		FullName: "Maria Santos",
		Email:    "not-an-email",
	})

	assert.Nil(t, staff)
	assert.EqualError(t, err, "email must be a valid email address")
}
