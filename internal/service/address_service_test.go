package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type addressRepoStub struct {
	created *model.Address
}

func (r *addressRepoStub) Create(_ context.Context, address *model.Address) error {
	r.created = address
	return nil
}

func (*addressRepoStub) GetByID(context.Context, int64, int64) (*model.Address, error) {
	return nil, errors.New("not implemented")
}

func (*addressRepoStub) GetByIDUnsafe(context.Context, int64) (*model.Address, error) {
	return nil, errors.New("not implemented")
}

func (*addressRepoStub) ListForUser(context.Context, int64, bool) ([]model.Address, error) {
	return nil, errors.New("not implemented")
}

func (*addressRepoStub) Update(context.Context, *model.Address) error {
	return errors.New("not implemented")
}

func (*addressRepoStub) SetDefault(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func (*addressRepoStub) SoftDelete(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func (*addressRepoStub) SetDisabled(context.Context, int64, int64, bool) error {
	return errors.New("not implemented")
}

type failingGeocoder struct{}

func (failingGeocoder) Geocode(context.Context, string) (*GeocodingResult, error) {
	return nil, errors.New("provider unavailable")
}

func (failingGeocoder) ReverseGeocode(context.Context, float64, float64) (*GeocodingResult, error) {
	return nil, errors.New("provider unavailable")
}

func TestAddressCreateFallsBackWhenGeocoderIsUnavailable(t *testing.T) {
	repo := &addressRepoStub{}
	service := NewAddressService(repo, failingGeocoder{})

	address, err := service.Create(context.Background(), 42, &model.CreateAddressRequest{
		Label:    "Home",
		Street:   "111 QA Test Street",
		Barangay: "South Triangle",
		City:     "Quezon City",
		Province: "Metro Manila",
		Country:  "Philippines",
	})
	if err != nil {
		t.Fatalf("expected address creation to continue without coordinates: %v", err)
	}
	if repo.created != address {
		t.Fatal("expected address to be persisted")
	}
	if address.Latitude != nil || address.Longitude != nil {
		t.Fatalf("expected nil fallback coordinates, got %v, %v", address.Latitude, address.Longitude)
	}
}
