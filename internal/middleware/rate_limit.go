package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	MaxAttempts     int           // Maximum failed attempts allowed
	LockoutDuration time.Duration // Duration to lock account after max attempts
	ResetWindow     time.Duration // Time window to reset attempt count
	CheckInterval   time.Duration // Cleanup interval for expired locks
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxAttempts:     5,
		LockoutDuration: 15 * time.Minute,
		ResetWindow:     1 * time.Hour,
		CheckInterval:   10 * time.Minute,
	}
}

type RateLimiter struct {
	db     db.DBTX
	config RateLimitConfig
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(db db.DBTX, config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		db:     db,
		config: config,
	}
	// Start cleanup goroutine
	go rl.cleanupExpiredLocks()
	return rl
}

// RateLimitAuthMiddleware wraps an auth handler with rate limiting
func (rl *RateLimiter) RateLimitAuthMiddleware(identifier string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check if identifier is locked
		locked, lockedUntil := rl.isLocked(ctx, identifier)
		if locked {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(lockedUntil).Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":        "Too many login attempts. Please try again later.",
				"retry_after":  int(time.Until(lockedUntil).Seconds()),
				"locked_until": lockedUntil.Format(time.RFC3339),
			})
			return
		}

		h.ServeHTTP(w, r)
	})
}

// RecordFailedAttempt records a failed login attempt
func (rl *RateLimiter) RecordFailedAttempt(ctx context.Context, identifier string) error {
	query := `
		INSERT INTO auth_rate_limits (identifier, attempt_count, first_attempt_at, last_attempt_at)
		VALUES ($1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (identifier) DO UPDATE SET
			attempt_count = auth_rate_limits.attempt_count + 1,
			last_attempt_at = CURRENT_TIMESTAMP,
			locked_until = CASE 
				WHEN (auth_rate_limits.attempt_count + 1) >= $2 
				THEN CURRENT_TIMESTAMP + ($3::interval)
				ELSE auth_rate_limits.locked_until
			END
	`
	_, err := rl.db.Exec(ctx, query, identifier, rl.config.MaxAttempts, fmt.Sprintf("%d seconds", int(rl.config.LockoutDuration.Seconds())))
	return err
}

// ResetAttempts resets the rate limit for successful login
func (rl *RateLimiter) ResetAttempts(ctx context.Context, identifier string) error {
	query := `
		DELETE FROM auth_rate_limits WHERE identifier = $1
	`
	_, err := rl.db.Exec(ctx, query, identifier)
	return err
}

// isLocked checks if an identifier is currently rate limited
func (rl *RateLimiter) isLocked(ctx context.Context, identifier string) (bool, time.Time) {
	query := `
		SELECT locked_until FROM auth_rate_limits 
		WHERE identifier = $1 AND locked_until IS NOT NULL AND locked_until > CURRENT_TIMESTAMP
	`
	var lockedUntil time.Time
	err := rl.db.QueryRow(ctx, query, identifier).Scan(&lockedUntil)
	if err != nil {
		return false, time.Time{}
	}
	return true, lockedUntil
}

// IsLocked is a public method to check rate limit status
func (rl *RateLimiter) IsLocked(ctx context.Context, identifier string) (bool, time.Time) {
	return rl.isLocked(ctx, identifier)
}

// cleanupExpiredLocks periodically cleans up expired rate limit records
func (rl *RateLimiter) cleanupExpiredLocks() {
	ticker := time.NewTicker(rl.config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		query := `
			DELETE FROM auth_rate_limits 
			WHERE (locked_until IS NULL AND last_attempt_at < CURRENT_TIMESTAMP - ($1::interval))
			   OR (locked_until IS NOT NULL AND locked_until < CURRENT_TIMESTAMP - ($2::interval))
		`
		rl.db.Exec(ctx, query, fmt.Sprintf("%d seconds", int(rl.config.ResetWindow.Seconds())), "30 minutes")
		cancel()
	}
}

// ExtractIdentifier extracts email, phone, or IP address from request
func ExtractIdentifier(r *http.Request, email, phone string) string {
	// Prefer email if provided
	if email != "" {
		return strings.ToLower(strings.TrimSpace(email))
	}
	// Then phone
	if phone != "" {
		return strings.TrimSpace(phone)
	}
	// Fall back to IP address
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return clientIP
}
