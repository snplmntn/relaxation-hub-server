package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type LiveLocationService struct {
    repo repository.LiveLocationRepository
}

func NewLiveLocationService(repo repository.LiveLocationRepository) *LiveLocationService {
    return &LiveLocationService{repo: repo}
}

func (s *LiveLocationService) UpdateLocation(ctx context.Context, userID int64, req *model.UpdateLocationRequest) (*model.LiveLocation, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    if req.Latitude < 5 || req.Latitude > 20 {
        return nil, fmt.Errorf("latitude out of PH range")
    }
    if req.Longitude < 116 || req.Longitude > 127 {
        return nil, fmt.Errorf("longitude out of PH range")
    }

    loc := &model.LiveLocation{
        UserID:    userID,
        Latitude:  req.Latitude,
        Longitude: req.Longitude,
    }

    if err := s.repo.Upsert(ctx, loc); err != nil {
        return nil, err
    }
    return loc, nil
}

func (s *LiveLocationService) GetByUserID(ctx context.Context, userID int64) (*model.LiveLocation, error) {
    return s.repo.GetByUserID(ctx, userID)
}
