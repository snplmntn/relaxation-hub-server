package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

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

	SMTP SMTPConfig

	BookingEmailTimezone string
	BookingDDayEmailHour int
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

	smtpPort := 0
	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("SMTP_PORT must be a number")
		}
		smtpPort = parsed
	}

	bookingDDayEmailHour := 7
	if raw := os.Getenv("BOOKING_DDAY_EMAIL_HOUR"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 23 {
			return nil, fmt.Errorf("BOOKING_DDAY_EMAIL_HOUR must be an hour from 0 to 23")
		}
		bookingDDayEmailHour = parsed
	}

	bookingEmailTimezone := os.Getenv("BOOKING_EMAIL_TIMEZONE")
	if bookingEmailTimezone == "" {
		bookingEmailTimezone = "Asia/Manila"
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

		SMTP: SMTPConfig{
			Host:      os.Getenv("SMTP_HOST"),
			Port:      smtpPort,
			Username:  os.Getenv("SMTP_USERNAME"),
			Password:  os.Getenv("SMTP_PASSWORD"),
			FromEmail: os.Getenv("SMTP_FROM_EMAIL"),
			FromName:  os.Getenv("SMTP_FROM_NAME"),
		},

		BookingEmailTimezone: bookingEmailTimezone,
		BookingDDayEmailHour: bookingDDayEmailHour,
	}, nil
}
