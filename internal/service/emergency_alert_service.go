package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type EmergencyAlertService struct {
    repo repository.EmergencyAlertRepository
}

func NewEmergencyAlertService(repo repository.EmergencyAlertRepository) *EmergencyAlertService {
    return &EmergencyAlertService{repo: repo}
}

func (s *EmergencyAlertService) Create(ctx context.Context, userID int64, req *model.CreateEmergencyAlertRequest) (*model.EmergencyAlert, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    if req.BookingID == 0 {
        return nil, fmt.Errorf("booking_id is required")
    }

    alert := &model.EmergencyAlert{
        BookingID:   req.BookingID,
        TriggeredBy: userID,
        LocationLat: req.LocationLat,
        LocationLng: req.LocationLng,
        Status:      "pending",
    }

    if err := s.repo.Create(ctx, alert); err != nil {
        return nil, err
    }
    return alert, nil
}

func (s *EmergencyAlertService) GetByID(ctx context.Context, alertID int64) (*model.EmergencyAlert, error) {
    return s.repo.GetByID(ctx, alertID)
}

func (s *EmergencyAlertService) Resolve(ctx context.Context, alertID, resolverID int64, req *model.ResolveEmergencyAlertRequest) (*model.EmergencyAlert, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    status := strings.TrimSpace(req.Status)
    if status == "" {
        return nil, fmt.Errorf("status is required")
    }

    if err := s.repo.Resolve(ctx, alertID, resolverID, status, strings.TrimSpace(req.ResolutionNote)); err != nil {
        return nil, err
    }
    return s.repo.GetByID(ctx, alertID)
}
