package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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

// PayMongoConfig holds credentials for online payment. When SecretKey is empty
// the online payment option is simply not offered — an unconfigured environment
// degrades to cash and manual transfer rather than erroring.
type PayMongoConfig struct {
	SecretKey     string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
	LiveMode      bool
}

// Enabled reports whether online payment can be offered.
func (p PayMongoConfig) Enabled() bool {
	return p.SecretKey != "" && p.SuccessURL != "" && p.CancelURL != ""
}

type Config struct {
	DatabaseURL            string
	JWTKey                 string
	Port                   string
	AutomatedOffersEnabled bool

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

	PayMongo PayMongoConfig
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

	// A configured key must carry a recognisable prefix: silently treating a
	// pasted-wrong value as live mode would verify webhooks against the wrong
	// signature and reject every real payment.
	paymongoSecret := strings.TrimSpace(os.Getenv("PAYMONGO_SECRET_KEY"))
	if paymongoSecret != "" && !strings.HasPrefix(paymongoSecret, "sk_test") && !strings.HasPrefix(paymongoSecret, "sk_live") {
		return nil, fmt.Errorf("PAYMONGO_SECRET_KEY must start with sk_test or sk_live")
	}

	bookingEmailTimezone := os.Getenv("BOOKING_EMAIL_TIMEZONE")
	if bookingEmailTimezone == "" {
		bookingEmailTimezone = "Asia/Manila"
	}

	automatedOffersEnabled := false
	if raw := os.Getenv("AUTOMATED_OFFERS_ENABLED"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("AUTOMATED_OFFERS_ENABLED must be true or false")
		}
		automatedOffersEnabled = parsed
	}

	return &Config{
		DatabaseURL:            dbURL,
		JWTKey:                 jwtKey,
		Port:                   port,
		AutomatedOffersEnabled: automatedOffersEnabled,

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

		// PayMongo (optional — absent means online payment is not offered)
		PayMongo: PayMongoConfig{
			SecretKey:     paymongoSecret,
			WebhookSecret: os.Getenv("PAYMONGO_WEBHOOK_SECRET"),
			SuccessURL:    os.Getenv("PAYMONGO_SUCCESS_URL"),
			CancelURL:     os.Getenv("PAYMONGO_CANCEL_URL"),
			LiveMode:      strings.HasPrefix(paymongoSecret, "sk_live"),
		},
	}, nil
}
