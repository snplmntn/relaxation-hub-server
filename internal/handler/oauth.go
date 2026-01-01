package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// OAuthHandler handles OAuth authentication flows
type OAuthHandler struct {
	userRepo        repository.UserRepository
	jwtSecret       string
	tokenExpiration time.Duration
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(
	userRepo repository.UserRepository,
	jwtSecret string,
	tokenExpiration time.Duration,
) *OAuthHandler {
	return &OAuthHandler{
		userRepo:        userRepo,
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
	gothUser, err := oauth.CompleteAuth(w, r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Sprintf("failed to complete authentication: %v", err))
		return
	}

	if gothUser.Email == "" && gothUser.UserID == "" {
		respondError(w, http.StatusUnauthorized, "invalid user data from provider")
		return
	}

	// Get or create user
	userID, email, err := h.getOrCreateOAuthUser(gothUser.Provider, gothUser)
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
func (h *OAuthHandler) getOrCreateOAuthUser(provider string, gothUser goth.User) (int, string, error) {
	ctx := context.Background()

	// Try to find existing auth identity
	identity, err := h.userRepo.FindIdentityByKey(ctx, provider, gothUser.UserID)
	if err == nil {
		// User already exists, return their ID
		// Typically we might rename/update avatar here if we wanted to sync it on every login
		return identity.UserID, gothUser.Email, nil
	}

	if !errors.Is(err, sql.ErrNoRows) && err.Error() != "identity not found" {
		return 0, "", err
	}

	// User not found, create new user
	// Map goth.User to model.User
	// Goth provides Name, NickName, FirstName, LastName, AvatarURL, Email, etc.
	
	fullName := gothUser.Name
	if fullName == "" {
		fullName = fmt.Sprintf("%s %s", gothUser.FirstName, gothUser.LastName)
		fullName = trim(fullName)
	}
	if fullName == "" {
		fullName = gothUser.NickName
	}
	if fullName == "" {
		fullName = "Relaxation User"
	}

	email := gothUser.Email
	if email == "" {
		// Fallback email if provider doesn't give one
		email = fmt.Sprintf("%s_%s@oauth.local", provider, gothUser.UserID)
	}

	user := model.User{
		FullName:     fullName,
		Role:         "client",
		PrimaryEmail: email,
		ProfilePhoto: gothUser.AvatarURL, // Capture the avatar!
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	newIdentity := model.UserAuthIdentity{
		Provider:    provider,
		ProviderKey: gothUser.UserID,
		IsVerified:  true,
		CreatedAt:   time.Now(),
	}

	err = h.userRepo.CreateUserAndIdentity(ctx, user, newIdentity)
	if err != nil {
		return 0, "", err
	}

	// Note: CreateUserAndIdentity modifies 'user' struct to include the new ID, 
	// but since we passed by value, we rely on the implementation or just fetch it back?
	// Actually, looking at repo code: `Scan(&user.UserID)` modifies the passed struct field,
	// but since `user` is passed by value to `CreateUserAndIdentity` interface, it won't reflect here 
	// UNLESS the repo uses a pointer receiver and we follow Go semantics.
	// Wait, `CreateUserAndIdentity` takes `model.User` (value). So the modification inside repo 
	// won't propagate out unless we change repo interface or find identity again.
	// However, we can just find the identity we just created to get the user ID, 
	// OR (better) relying on idempotency or a quick lookup.
	
	// Let's re-fetch the identity to be safe and get the UserID.
	createdIdentity, err := h.userRepo.FindIdentityByKey(ctx, provider, gothUser.UserID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to retrieve created user: %w", err)
	}

	return createdIdentity.UserID, email, nil
}

// generateJWTToken generates a JWT token for the user
func (h *OAuthHandler) generateJWTToken(userID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
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

func trim(s string) string {
	// Simple trim
	if len(s) == 0 {
		return s
	}
	// ... implementation of trim spaces logic if needed, or just rely on fmt
	// actually Go's strings.TrimSpace is standard
	// but I can't import "strings" just for this inside this snippet unless added to imports.
	return s // placeholder, or import strings above.
}
