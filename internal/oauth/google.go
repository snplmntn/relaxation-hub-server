package oauth

import (
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"
)

// NewGoogleProvider creates a configured Google OAuth provider
func NewGoogleProvider(clientID, clientSecret, callbackURL string) goth.Provider {
	return google.New(clientID, clientSecret, callbackURL)
}
