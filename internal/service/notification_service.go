package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type NotificationService struct {
    repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) *NotificationService {
    return &NotificationService{repo: repo}
}

func (s *NotificationService) Create(ctx context.Context, req *model.CreateNotificationRequest) (*model.Notification, error) {
    if req == nil {
        return nil, fmt.Errorf("request is required")
    }
    if req.UserID == 0 {
        return nil, fmt.Errorf("user_id is required")
    }
    notifType := strings.TrimSpace(req.Type)
    if notifType == "" {
        return nil, fmt.Errorf("type is required")
    }

    n := &model.Notification{
        UserID:  req.UserID,
        Type:    notifType,
        Title:   strings.TrimSpace(req.Title),
        Message: strings.TrimSpace(req.Message),
    }

    if err := s.repo.Create(ctx, n); err != nil {
        return nil, err
    }
    return n, nil
}

func (s *NotificationService) ListByUser(ctx context.Context, userID int64) ([]model.Notification, error) {
    return s.repo.ListByUser(ctx, userID)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
    return s.repo.MarkAsRead(ctx, notificationID, userID)
}
