package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

type LiveLocationService struct {
    repo repository.LiveLocationRepository
    hub  *ws.Hub
}

func NewLiveLocationService(repo repository.LiveLocationRepository, hub *ws.Hub) *LiveLocationService {
    return &LiveLocationService{repo: repo, hub: hub}
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

    // Broadcast location update to subscribers (can be extended to notify specific users)
    // For now, we could notify all users tracking this therapist
    s.hub.SendToUser(userID, "location_update", loc)

    return loc, nil
}

func (s *LiveLocationService) GetByUserID(ctx context.Context, userID int64) (*model.LiveLocation, error) {
    return s.repo.GetByUserID(ctx, userID)
}
