package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const pushConcurrencyLimit = 20 // Max concurrent push notification goroutines

type NotificationService struct {
	repo     repository.NotificationRepository
	userRepo repository.UserRepository
	fcm      *FCMService
	pushSem  chan struct{} // Semaphore to limit push notification concurrency
}

func NewNotificationService(repo repository.NotificationRepository, userRepo repository.UserRepository, fcm *FCMService) *NotificationService {
	return &NotificationService{
		repo:     repo,
		userRepo: userRepo,
		fcm:      fcm,
		pushSem:  make(chan struct{}, pushConcurrencyLimit),
	}
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
		Data:           req.Data,
	})

	// Send push notification via FCM with concurrency limit
	if s.fcm != nil && s.userRepo != nil {
		select {
		case s.pushSem <- struct{}{}: // Acquire semaphore slot
			go func() {
				defer func() { <-s.pushSem }() // Release semaphore slot
				s.sendPushNotification(context.WithoutCancel(ctx), n)
			}()
		default:
			// Semaphore full, drop the push notification (non-blocking fallback)
			slog.Warn("push semaphore full, dropping FCM push", "user_id", n.UserID)
		}
	}

	return n, nil
}

// SendPushDirect sends a push notification immediately without persisting it to the database.
func (s *NotificationService) SendPushDirect(ctx context.Context, userID int64, notifType, title, message string, data map[string]string) {
	if s.fcm == nil || s.userRepo == nil {
		return
	}

	fcmToken, err := s.userRepo.GetFCMToken(ctx, userID)
	if err != nil {
		slog.Warn("SendPushDirect: failed to get FCM token", "user_id", userID, "error", err)
		return
	}
	if fcmToken == nil || *fcmToken == "" {
		slog.Debug("SendPushDirect: no FCM token registered", "user_id", userID)
		return
	}

	if data == nil {
		data = make(map[string]string)
	}
	data["type"] = notifType

	if err := s.fcm.SendNotification(ctx, *fcmToken, title, message, data); err != nil {
		slog.Warn("SendPushDirect: failed to send FCM notification", "user_id", userID, "error", err)
	} else {
		slog.Debug("SendPushDirect: FCM notification sent", "user_id", userID)
	}
}

// sendPushNotification fetches the user's FCM token and sends a push notification.
func (s *NotificationService) sendPushNotification(ctx context.Context, n *model.Notification) {
	fcmToken, err := s.userRepo.GetFCMToken(ctx, n.UserID)
	if err != nil {
		slog.Warn("failed to get FCM token", "user_id", n.UserID, "error", err)
		return
	}
	if fcmToken == nil || *fcmToken == "" {
		slog.Debug("no FCM token registered", "user_id", n.UserID)
		return
	}

	data := make(map[string]string)
	if n.Data != nil {
		var rawData map[string]interface{}
		if err := json.Unmarshal(n.Data, &rawData); err == nil {
			for k, v := range rawData {
				data[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	data["notification_id"] = fmt.Sprintf("%d", n.NotificationID)
	data["type"] = n.Type

	if err := s.fcm.SendNotification(ctx, *fcmToken, n.Title, n.Message, data); err != nil {
		slog.Warn("failed to send FCM notification", "user_id", n.UserID, "error", err)
	} else {
		slog.Debug("FCM notification sent", "user_id", n.UserID)
	}
}

func (s *NotificationService) ListByUser(ctx context.Context, userID int64, limit, offset int) (*model.PaginatedNotificationsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	notifs, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	totalPages := (total + limit - 1) / limit
	hasMore := (offset + limit) < total
	page := (offset / limit) + 1

	out := make([]model.NotificationResponse, 0, len(notifs))
	for i := range notifs {
		n := &notifs[i]
		var respData map[string]any
		if len(n.Data) > 0 {
			_ = json.Unmarshal(n.Data, &respData)
		}
		out = append(out, model.NotificationResponse{
			NotificationID: n.NotificationID,
			Type:           n.Type,
			Title:          n.Title,
			Message:        n.Message,
			IsRead:         n.IsRead,
			ReadAt:         n.ReadAt,
			CreatedAt:      n.CreatedAt,
			UpdatedAt:      n.UpdatedAt,
			Data:           respData,
		})
	}

	return &model.PaginatedNotificationsResponse{
		Notifications: out,
		Total:         total,
		Page:          page,
		Limit:         limit,
		TotalPages:    totalPages,
		HasMore:       hasMore,
	}, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return s.repo.MarkAsRead(ctx, notificationID, userID)
}
