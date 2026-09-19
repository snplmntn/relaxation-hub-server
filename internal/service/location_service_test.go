package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type stubServiceAreaRepo struct {
	areaByName     *model.ServiceArea
	areasByName    map[string]*model.ServiceArea
	getByNameErr   error
	recordErr      error
	upsertedAreas  []model.ServiceArea
	recordedAreaID []string
}

func (s *stubServiceAreaRepo) GetByKey(context.Context, string) (*model.ServiceArea, error) {
	return nil, repository.ErrAreaNotFound
}

func (s *stubServiceAreaRepo) GetByName(_ context.Context, name string, level model.ServiceAreaLevel) (*model.ServiceArea, error) {
	if s.getByNameErr != nil {
		return nil, s.getByNameErr
	}
	if len(s.areasByName) > 0 {
		if area, ok := s.areasByName[stubAreaNameKey(level, name)]; ok {
			return area, nil
		}
		return nil, repository.ErrAreaNotFound
	}
	if s.areaByName != nil {
		return s.areaByName, nil
	}
	return nil, repository.ErrAreaNotFound
}

func stubAreaNameKey(level model.ServiceAreaLevel, name string) string {
	return string(level) + ":" + strings.ToLower(strings.TrimSpace(name))
}

func (s *stubServiceAreaRepo) GetStatusByKey(context.Context, string) (model.ServiceAreaStatus, error) {
	return model.ServiceAreaStatusNotSupported, nil
}

func (s *stubServiceAreaRepo) ListByStatus(context.Context, model.ServiceAreaStatus) ([]model.ServiceArea, error) {
	return nil, nil
}

func (s *stubServiceAreaRepo) ListAll(context.Context) ([]model.ServiceArea, error) {
	return nil, nil
}

func (s *stubServiceAreaRepo) ListTopDemand(context.Context, int) ([]model.ServiceArea, error) {
	return nil, nil
}

func (s *stubServiceAreaRepo) UpdateStatus(context.Context, string, model.ServiceAreaStatus) error {
	return nil
}

func (s *stubServiceAreaRepo) UpsertArea(_ context.Context, area *model.ServiceArea) error {
	s.upsertedAreas = append(s.upsertedAreas, *area)
	return nil
}

func (s *stubServiceAreaRepo) RecordInterest(_ context.Context, _ int64, areaKey string) error {
	s.recordedAreaID = append(s.recordedAreaID, areaKey)
	return s.recordErr
}

func (s *stubServiceAreaRepo) GetInterestCount(context.Context, string) (int, error) {
	return 0, nil
}

func (s *stubServiceAreaRepo) ListInterestedUsers(context.Context, string) ([]int64, error) {
	return nil, nil
}

func (s *stubServiceAreaRepo) ListInterestedUsersPage(context.Context, string, int, int) ([]model.AreaInterestedUser, int, error) {
	return nil, 0, nil
}

func TestLocationServiceCheckLocationByName_PropagatesInterestRecordError(t *testing.T) {
	t.Parallel()

	recordErr := errors.New("trigger failed")
	repo := &stubServiceAreaRepo{recordErr: recordErr}
	svc := NewLocationService(repo)

	_, err := svc.CheckLocationByName(context.Background(), 42, "Calamba", "Barangay Uno")
	if !errors.Is(err, recordErr) {
		t.Fatalf("expected record interest error, got %v", err)
	}

	if len(repo.upsertedAreas) != 1 {
		t.Fatalf("expected unknown area to be upserted once, got %d", len(repo.upsertedAreas))
	}
	if len(repo.recordedAreaID) != 1 {
		t.Fatalf("expected interest recording to be attempted once, got %d", len(repo.recordedAreaID))
	}
}

func TestLocationServiceCheckLocationByName_AllowsDuplicateInterest(t *testing.T) {
	t.Parallel()

	repo := &stubServiceAreaRepo{recordErr: repository.ErrDuplicateInterest}
	svc := NewLocationService(repo)

	result, err := svc.CheckLocationByName(context.Background(), 42, "Calamba", "Barangay Uno")
	if err != nil {
		t.Fatalf("expected duplicate interest to be tolerated, got %v", err)
	}

	if result == nil || result.IsAllowed {
		t.Fatalf("expected unsupported result, got %#v", result)
	}
}

func TestLocationServiceCheckLocationByName_AllowsCoveredCityWhenBarangayIsPending(t *testing.T) {
	t.Parallel()

	repo := &stubServiceAreaRepo{
		areasByName: map[string]*model.ServiceArea{
			stubAreaNameKey(model.ServiceAreaLevelBarangay, "Poblacion"): {
				AreaKey:           "barangay:poblacion|city:makati",
				Name:              "Poblacion",
				Level:             model.ServiceAreaLevelBarangay,
				Status:            model.ServiceAreaStatusNotSupported,
				MinBookingMinutes: 60,
			},
			stubAreaNameKey(model.ServiceAreaLevelCity, "Makati"): {
				AreaKey:           "city:makati",
				Name:              "Makati",
				Level:             model.ServiceAreaLevelCity,
				Status:            model.ServiceAreaStatusCovered,
				MinBookingMinutes: 90,
			},
		},
	}
	svc := NewLocationService(repo)

	result, err := svc.CheckLocationByName(context.Background(), 42, "Makati", "Poblacion")
	if err != nil {
		t.Fatalf("expected covered city to allow the address, got error %v", err)
	}

	if result == nil || !result.IsAllowed {
		t.Fatalf("expected covered result, got %#v", result)
	}
	if result.AreaKey != "city:makati" {
		t.Fatalf("expected city area key, got %q", result.AreaKey)
	}
	if result.MinBooking != 90 {
		t.Fatalf("expected city min booking to be used, got %d", result.MinBooking)
	}
	if len(repo.recordedAreaID) != 0 {
		t.Fatalf("expected no demand record for a covered city, got %v", repo.recordedAreaID)
	}
}

func TestLocationServiceCheckLocationByCoordinatesForArea_ReturnsNotSupportedWhenInterestRecordingFails(t *testing.T) {
	t.Parallel()

	recordErr := errors.New("record interest failed")
	repo := &stubServiceAreaRepo{recordErr: recordErr}
	svc := NewLocationService(repo)

	result, err := svc.CheckLocationByCoordinatesForArea(context.Background(), 42, 16.4023, 120.5960, "Baguio", "")
	if err != nil {
		t.Fatalf("expected interest recording failure to keep location response non-fatal, got %v", err)
	}
	if result == nil || result.IsAllowed {
		t.Fatalf("expected unsupported result, got %#v", result)
	}
	if result.Status != model.ServiceAreaStatusNotSupported {
		t.Fatalf("expected not_supported status, got %q", result.Status)
	}
	if result.AreaKey != "city:baguio" {
		t.Fatalf("expected demand area key to be returned, got %q", result.AreaKey)
	}
}
