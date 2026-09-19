package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/idtoken"
)

var ErrGoogleCredentialNotConfigured = errors.New("google credential authentication is not configured")

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type GoogleCredentialVerifier interface {
	Verify(ctx context.Context, credential string) (*GoogleIdentity, error)
}

type googleCredentialVerifier struct {
	clientID string
}

func NewGoogleCredentialVerifier(clientID string) GoogleCredentialVerifier {
	return &googleCredentialVerifier{clientID: strings.TrimSpace(clientID)}
}

func (v *googleCredentialVerifier) Verify(ctx context.Context, credential string) (*GoogleIdentity, error) {
	if v.clientID == "" {
		return nil, ErrGoogleCredentialNotConfigured
	}
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return nil, errors.New("google credential is required")
	}

	payload, err := idtoken.Validate(ctx, credential, v.clientID)
	if err != nil {
		return nil, fmt.Errorf("validate google credential: %w", err)
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return nil, errors.New("invalid google credential issuer")
	}

	identity := &GoogleIdentity{
		Subject: payload.Subject,
		Email:   stringClaim(payload.Claims, "email"),
		Name:    stringClaim(payload.Claims, "name"),
		Picture: stringClaim(payload.Claims, "picture"),
	}
	identity.EmailVerified, _ = payload.Claims["email_verified"].(bool)
	if identity.Subject == "" || identity.Email == "" {
		return nil, errors.New("google credential is missing required claims")
	}
	return identity, nil
}

func stringClaim(claims map[string]interface{}, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}
