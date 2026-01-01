package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type NotificationService struct {
	repo     repository.NotificationRepository
	userRepo repository.UserRepository
	fcm      *FCMService
}

func NewNotificationService(repo repository.NotificationRepository, userRepo repository.UserRepository, fcm *FCMService) *NotificationService {
	return &NotificationService{repo: repo, userRepo: userRepo, fcm: fcm}
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

	var dataBytes []byte
	if req.Data != nil {
		b, err := json.Marshal(req.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		dataBytes = b
	}

	n := &model.Notification{
		UserID:  req.UserID,
		Type:    notifType,
		Title:   strings.TrimSpace(req.Title),
		Message: strings.TrimSpace(req.Message),
		Data:    dataBytes,
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}

	// Broadcast notification in real-time via WebSocket
	_ = broadcaster.BroadcastToUser(n.UserID, "notification:created", model.NotificationResponse{
		NotificationID: n.NotificationID,
		Type:           n.Type,
		Title:          n.Title,
		Message:        n.Message,
		IsRead:         n.IsRead,
		ReadAt:         n.ReadAt,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	})

	// Send push notification via FCM
	if s.fcm != nil && s.userRepo != nil {
		go s.sendPushNotification(context.WithoutCancel(ctx), n)
	}

	return n, nil
}

// sendPushNotification fetches the user's FCM token and sends a push notification.
func (s *NotificationService) sendPushNotification(ctx context.Context, n *model.Notification) {
	fcmToken, err := s.userRepo.GetFCMToken(ctx, n.UserID)
	if err != nil {
		log.Printf("Failed to get FCM token for user %d: %v", n.UserID, err)
		return
	}
	if fcmToken == nil || *fcmToken == "" {
		log.Printf("User %d has no FCM token registered", n.UserID)
		return
	}

	data := make(map[string]string)
	if n.Data != nil {
		_ = json.Unmarshal(n.Data, &data)
	}
	data["notification_id"] = fmt.Sprintf("%d", n.NotificationID)
	data["type"] = n.Type

	if err := s.fcm.SendNotification(ctx, *fcmToken, n.Title, n.Message, data); err != nil {
		log.Printf("Failed to send FCM notification to user %d: %v", n.UserID, err)
	} else {
		log.Printf("FCM notification sent to user %d", n.UserID)
	}
}

func (s *NotificationService) ListByUser(ctx context.Context, userID int64) ([]model.Notification, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return s.repo.MarkAsRead(ctx, notificationID, userID)
}
