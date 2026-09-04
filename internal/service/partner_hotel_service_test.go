package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type partnerHotelServiceRepo struct {
	hotel        *model.PartnerHotel
	createdStaff *model.PartnerHotelStaff
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

func (r *partnerHotelServiceRepo) ListStaff(_ context.Context, _ int64) ([]model.PartnerHotelStaff, error) {
	return nil, nil
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
