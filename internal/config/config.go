package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTKey      string
	Port        string

	// OAuth Google
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	GoogleOAuthCallbackURL  string

	// OAuth Apple
	AppleOAuthClientID     string
	AppleOAuthClientSecret string
	AppleOAuthCallbackURL  string
	AppleOAuthTeamID       string
	AppleOAuthKeyID        string
	AppleOAuthPrivateKey   string

	// AWS S3
	AWSS3Bucket string
	AWSRegion   string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		return nil, fmt.Errorf("JWT_KEY environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		return nil, fmt.Errorf("PORT environment variable is required")
	}

	return &Config{
		DatabaseURL: dbURL,
		JWTKey:      jwtKey,
		Port:        port,

		// OAuth Google (optional)
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleOAuthCallbackURL:  os.Getenv("GOOGLE_OAUTH_CALLBACK_URL"),

		// OAuth Apple (optional)
		AppleOAuthClientID:     os.Getenv("APPLE_OAUTH_CLIENT_ID"),
		AppleOAuthClientSecret: os.Getenv("APPLE_OAUTH_CLIENT_SECRET"),
		AppleOAuthCallbackURL:  os.Getenv("APPLE_OAUTH_CALLBACK_URL"),
		AppleOAuthTeamID:       os.Getenv("APPLE_OAUTH_TEAM_ID"),
		AppleOAuthKeyID:        os.Getenv("APPLE_OAUTH_KEY_ID"),
		AppleOAuthPrivateKey:   os.Getenv("APPLE_OAUTH_PRIVATE_KEY"),

		// AWS S3 (optional)
		AWSS3Bucket: os.Getenv("AWS_S3_BUCKET"),
		AWSRegion:   os.Getenv("AWS_REGION"),
	}, nil
}
