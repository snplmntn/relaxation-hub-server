package config

// OAuthConfig holds OAuth provider credentials
type OAuthConfig struct {
	Google *OAuthProviderConfig `mapstructure:"google"`
	Apple  *OAuthProviderConfig `mapstructure:"apple"`
}

// OAuthProviderConfig holds individual provider configuration
type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	CallbackURL  string `mapstructure:"callback_url"`
	TeamID       string `mapstructure:"team_id"`     // Apple only
	KeyID        string `mapstructure:"key_id"`      // Apple only
	PrivateKey   string `mapstructure:"private_key"` // Apple only
}
