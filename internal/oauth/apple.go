package oauth

import (
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/apple"
)

// NewAppleProvider creates a configured Apple OAuth provider
func NewAppleProvider(clientID, clientSecret, callbackURL string) goth.Provider {
	return apple.New(clientID, clientSecret, callbackURL, nil, apple.ScopeName, apple.ScopeEmail)
}
