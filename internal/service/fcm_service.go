package service

import (
	"context"
	"fmt"
	"os"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMService handles sending push notifications via Firebase Cloud Messaging.
type FCMService struct {
	client *messaging.Client
	mu     sync.RWMutex
}

var (
	fcmInstance *FCMService
	fcmOnce     sync.Once
)

// NewFCMService creates and initializes the FCM service singleton.
// It looks for the service account key at the path specified by
// FIREBASE_SERVICE_ACCOUNT_PATH env var, defaulting to "firebase-service-account.json".
func NewFCMService(ctx context.Context) (*FCMService, error) {
	var initErr error
	fcmOnce.Do(func() {
		keyPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH")
		if keyPath == "" {
			keyPath = "firebase-service-account.json"
		}

		opt := option.WithCredentialsFile(keyPath)
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize Firebase app: %w", err)
			return
		}

		client, err := app.Messaging(ctx)
		if err != nil {
			initErr = fmt.Errorf("failed to get Firebase messaging client: %w", err)
			return
		}

		fcmInstance = &FCMService{client: client}
	})

	if initErr != nil {
		return nil, initErr
	}
	return fcmInstance, nil
}

// SendNotification sends a push notification to a specific device token.
func (f *FCMService) SendNotification(ctx context.Context, token, title, body string, data map[string]string) error {
	if token == "" {
		return fmt.Errorf("FCM token is empty")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "relaxation_hub_notifications",
				Sound:     "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
					Badge: func() *int { i := 1; return &i }(),
				},
			},
		},
	}

	respID, err := f.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send FCM message: %w", err)
	}
    // This will be logged by the caller, but useful to return or log here if needed
    // The caller (notification_service) logs "FCM notification sent..."
    // We can add a debug log here if we had a logger, but we don't have one injected.
    // Instead, modify the return to include ID? No, interface is fixed.
    // Let's just print to stdout for now for debugging
    fmt.Printf("DEBUG: FCM successfully sent message: %s\n", respID)

	return nil
}

// SendToMultiple sends a notification to multiple device tokens.
func (f *FCMService) SendToMultiple(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	_, err := f.client.SendEachForMulticast(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send multicast FCM message: %w", err)
	}

	return nil
}
