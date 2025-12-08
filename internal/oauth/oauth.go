package oauth

import (
	"fmt"
	"net/http"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

// OAuthProvider represents configured OAuth providers
type OAuthProvider struct {
	Google *GoogleConfig
	Apple  *AppleConfig
}

// GoogleConfig for Google OAuth
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// AppleConfig for Apple OAuth
type AppleConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// InitGothProviders initializes all configured OAuth providers
func InitGothProviders(cfg *OAuthProvider) error {
	if cfg == nil {
		return fmt.Errorf("oauth config is required")
	}

	var providers []goth.Provider

	// Initialize Google provider if configured
	if cfg.Google != nil && cfg.Google.ClientID != "" {
		providers = append(providers, NewGoogleProvider(
			cfg.Google.ClientID,
			cfg.Google.ClientSecret,
			cfg.Google.CallbackURL,
		))
	}

	// Initialize Apple provider if configured
	if cfg.Apple != nil && cfg.Apple.ClientID != "" {
		providers = append(providers, NewAppleProvider(
			cfg.Apple.ClientID,
			cfg.Apple.ClientSecret,
			cfg.Apple.CallbackURL,
		))
	}

	if len(providers) > 0 {
		goth.UseProviders(providers...)
	}

	return nil
}

// BeginAuth initiates OAuth login flow
func BeginAuth(w http.ResponseWriter, r *http.Request) error {
	gothic.BeginAuthHandler(w, r)
	return nil
}

// CompleteAuth completes the OAuth flow and returns user info
func CompleteAuth(w http.ResponseWriter, r *http.Request) (goth.User, error) {
	return gothic.CompleteUserAuth(w, r)
}
