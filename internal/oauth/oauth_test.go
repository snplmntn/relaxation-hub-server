package oauth

import (
	"testing"
)

func TestNewGoogleProvider(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	callbackURL := "http://localhost:8080/oauth/callback"

	provider := NewGoogleProvider(clientID, clientSecret, callbackURL)

	if provider == nil {
		t.Error("Expected provider, got nil")
	}

	if provider.Name() != "google" {
		t.Errorf("Expected provider name 'google', got %s", provider.Name())
	}
}

func TestNewAppleProvider(t *testing.T) {
	clientID := "com.example.app"
	clientSecret := "test-secret"
	callbackURL := "http://localhost:8080/oauth/callback"

	provider := NewAppleProvider(clientID, clientSecret, callbackURL)

	if provider == nil {
		t.Error("Expected provider, got nil")
	}

	if provider.Name() != "apple" {
		t.Errorf("Expected provider name 'apple', got %s", provider.Name())
	}
}
