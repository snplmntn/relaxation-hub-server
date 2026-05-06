package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type stubServiceAreaRepo struct {
	areaByName     *model.ServiceArea
	getByNameErr   error
	recordErr      error
	upsertedAreas  []model.ServiceArea
	recordedAreaID []string
}

func (s *stubServiceAreaRepo) GetByKey(context.Context, string) (*model.ServiceArea, error) {
	return nil, repository.ErrAreaNotFound
}

func (s *stubServiceAreaRepo) GetByName(context.Context, string, model.ServiceAreaLevel) (*model.ServiceArea, error) {
	if s.getByNameErr != nil {
		return nil, s.getByNameErr
	}
	if s.areaByName != nil {
		return s.areaByName, nil
	}
	return nil, repository.ErrAreaNotFound
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
