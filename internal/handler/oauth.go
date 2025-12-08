package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markbates/goth"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
)

// OAuthHandler handles OAuth authentication flows
type OAuthHandler struct {
	pool            *pgxpool.Pool
	jwtSecret       string
	tokenExpiration time.Duration
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(
	pool *pgxpool.Pool,
	jwtSecret string,
	tokenExpiration time.Duration,
) *OAuthHandler {
	return &OAuthHandler{
		pool:            pool,
		jwtSecret:       jwtSecret,
		tokenExpiration: tokenExpiration,
	}
}

// OAuthLoginRequest initiates OAuth login
// GET /api/v1/oauth/:provider
func (h *OAuthHandler) OAuthLoginRequest(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	if provider == "" {
		respondError(w, http.StatusBadRequest, "provider is required")
		return
	}

	// Let gothic handle the OAuth flow
	oauth.BeginAuth(w, r)
}

// OAuthCallbackRequest handles OAuth provider callback
// GET /api/v1/oauth/callback
func (h *OAuthHandler) OAuthCallbackRequest(w http.ResponseWriter, r *http.Request) {
	// Complete the OAuth flow using gothic
	user, err := oauth.CompleteAuth(w, r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Sprintf("failed to complete authentication: %v", err))
		return
	}

	if user.Email == "" && user.UserID == "" {
		respondError(w, http.StatusUnauthorized, "invalid user data from provider")
		return
	}

	// Get or create user
	userID, email, err := h.getOrCreateOAuthUser(user.Provider, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to process authentication")
		return
	}

	// Generate JWT token
	token, err := h.generateJWTToken(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	respondSuccess(w, "oauth_login_successful", map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    userID,
			"email": email,
		},
	})
}

// getOrCreateOAuthUser retrieves or creates a user from OAuth provider data
func (h *OAuthHandler) getOrCreateOAuthUser(provider string, user goth.User) (uuid.UUID, string, error) {
	// Try to find existing auth identity
	userID, err := h.findUserByAuthIdentity(provider, user.UserID)
	if err == nil {
		// User already exists, return their ID
		return userID, user.Email, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, "", err
	}

	// Create new user
	newUserID, err := h.createOAuthUser(provider, user)
	if err != nil {
		return uuid.Nil, "", err
	}

	return newUserID, user.Email, nil
}

// findUserByAuthIdentity looks up user by OAuth identity
func (h *OAuthHandler) findUserByAuthIdentity(provider, providerKey string) (uuid.UUID, error) {
	var userID uuid.UUID

	err := h.pool.QueryRow(context.Background(),
		`SELECT user_id FROM user_auth_identities 
		 WHERE provider = $1 AND provider_key = $2`,
		provider, providerKey,
	).Scan(&userID)

	return userID, err
}

// createOAuthUser creates a new user and auth identity
func (h *OAuthHandler) createOAuthUser(provider string, user goth.User) (uuid.UUID, error) {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	userID := uuid.New()
	email := user.Email
	if email == "" {
		email = fmt.Sprintf("%s_%s@oauth.local", provider, user.UserID)
	}

	// Create user
	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, email, first_name, last_name, phone, role, is_email_verified, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, '', 'client', true, NOW(), NOW())`,
		userID, email, user.FirstName, user.LastName,
	)
	if err != nil {
		return uuid.Nil, err
	}

	// Create auth identity
	_, err = tx.Exec(ctx,
		`INSERT INTO user_auth_identities (id, user_id, provider, provider_key, is_verified, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, true, NOW(), NOW())`,
		uuid.New(), userID, provider, user.UserID,
	)
	if err != nil {
		return uuid.Nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

// generateJWTToken generates a JWT token for the user
func (h *OAuthHandler) generateJWTToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"role":    "client",
		"exp":     time.Now().Add(h.tokenExpiration).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// OAuthLogout handles OAuth logout
// POST /api/v1/oauth/logout
func (h *OAuthHandler) OAuthLogout(w http.ResponseWriter, r *http.Request) {
	// OAuth logout is typically handled client-side or via provider revocation
	// This endpoint is informational
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	respondSuccess(w, "logout_successful", nil)
}
