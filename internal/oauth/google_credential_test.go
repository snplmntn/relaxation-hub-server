package oauth

import (
	"context"
	"errors"
	"testing"
)

func TestGoogleCredentialVerifierRequiresConfiguration(t *testing.T) {
	verifier := NewGoogleCredentialVerifier("")
	_, err := verifier.Verify(context.Background(), "credential")
	if !errors.Is(err, ErrGoogleCredentialNotConfigured) {
		t.Fatalf("expected configuration error, got %v", err)
	}
}

func TestGoogleCredentialVerifierRequiresCredential(t *testing.T) {
	verifier := NewGoogleCredentialVerifier("client-id")
	if _, err := verifier.Verify(context.Background(), "  "); err == nil {
		t.Fatal("expected missing credential error")
	}
}
