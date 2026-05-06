package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
		// Check if we have env vars for credentials
		privateKey := os.Getenv("FIREBASE_PRIVATE_KEY")
		if privateKey != "" {
			// Construct JSON from env vars
			// Handle potential escaped newlines in private key
			privateKey = strings.ReplaceAll(privateKey, "\\n", "\n")

			credsMap := map[string]string{
				"type":                        os.Getenv("FIREBASE_TYPE"),
				"project_id":                  os.Getenv("FIREBASE_PROJECT_ID"),
				"private_key_id":              os.Getenv("FIREBASE_PRIVATE_KEY_ID"),
				"private_key":                 privateKey,
				"client_email":                os.Getenv("FIREBASE_CLIENT_EMAIL"),
				"client_id":                   os.Getenv("FIREBASE_CLIENT_ID"),
				"auth_uri":                    os.Getenv("FIREBASE_AUTH_URI"),
				"token_uri":                   os.Getenv("FIREBASE_TOKEN_URI"),
				"auth_provider_x509_cert_url": os.Getenv("FIREBASE_AUTH_PROVIDER_X509_CERT_URL"),
				"client_x509_cert_url":        os.Getenv("FIREBASE_CLIENT_X509_CERT_URL"),
				"universe_domain":             os.Getenv("FIREBASE_UNIVERSE_DOMAIN"),
			}

			// Fill defaults for standard Google fields if missing
			if credsMap["type"] == "" {
				credsMap["type"] = "service_account"
			}
			if credsMap["auth_uri"] == "" {
				credsMap["auth_uri"] = "https://accounts.google.com/o/oauth2/auth"
			}
			if credsMap["token_uri"] == "" {
				credsMap["token_uri"] = "https://oauth2.googleapis.com/token"
			}
			if credsMap["auth_provider_x509_cert_url"] == "" {
				credsMap["auth_provider_x509_cert_url"] = "https://www.googleapis.com/oauth2/v1/certs"
			}
			if credsMap["universe_domain"] == "" {
				credsMap["universe_domain"] = "googleapis.com"
			}

			jsonBytes, err := json.Marshal(credsMap)
			if err != nil {
				initErr = fmt.Errorf("failed to marshal firebase credentials from env: %w", err)
				return
			}

			opt := option.WithCredentialsJSON(jsonBytes)
			app, err := firebase.NewApp(ctx, nil, opt)
			if err != nil {
				initErr = fmt.Errorf("failed to initialize Firebase app from env: %w", err)
				return
			}

			client, err := app.Messaging(ctx)
			if err != nil {
				initErr = fmt.Errorf("failed to get Firebase messaging client: %w", err)
				return
			}
			fcmInstance = &FCMService{client: client}
			return
		}

		// Fallback to file
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

	_ = respID // Ignored for now

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
