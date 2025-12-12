package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AuthHandler struct {
	AuthService     service.AuthService
	RateLimiter     *middleware.RateLimiter
	ReferralService service.ReferralService
}

func NewAuthHandler(
	authService service.AuthService,
	rateLimiter *middleware.RateLimiter,
	referralService service.ReferralService,
) *AuthHandler {
	return &AuthHandler{
		AuthService:     authService,
		RateLimiter:     rateLimiter,
		ReferralService: referralService,
	}
}

type AuthRequest struct {
	FullName     string `json:"full_name"`
	Provider     string `json:"provider"`
	ProviderKey  string `json:"provider_key"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	ReferralCode string `json:"referral_code,omitempty"` // For signup with referral
}

type LoginRequest struct {
	Provider    string `json:"provider"`
	ProviderKey string `json:"provider_key"`
	Password    string `json:"password"`
}

func (h *AuthHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	userID, err := h.AuthService.Signup(r.Context(), req.FullName, req.Provider, req.ProviderKey, req.Password, req.Role)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "email already in use" {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Handle referral if provided
	if req.ReferralCode != "" && h.ReferralService != nil {
		err = h.ReferralService.CompleteReferralByCode(r.Context(), req.ReferralCode, int64(userID))
		if err != nil {
			// Log the error but don't fail the signup
			// This ensures signup succeeds even if referral processing fails
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Signed up successfully!",
		"user_id": userID,
	})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Extract identifier for rate limiting
	identifier := middleware.ExtractIdentifier(r, req.ProviderKey, "")
	if req.Provider == "email" {
		identifier = req.ProviderKey
	}

	// Check rate limiting
	if h.RateLimiter != nil {
		locked, lockedUntil := h.RateLimiter.IsLocked(r.Context(), identifier)
		if locked {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "900") // 15 minutes
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":        "Too many login attempts. Please try again later.",
				"retry_after":  900,
				"locked_until": lockedUntil.Format("2006-01-02T15:04:05Z07:00"),
			})
			return
		}
	}

	tokenString, err := h.AuthService.Login(r.Context(), req.Provider, req.ProviderKey, req.Password)
	if err != nil {
		// Record failed attempt for rate limiting
		if h.RateLimiter != nil {
			h.RateLimiter.RecordFailedAttempt(r.Context(), identifier)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Reset rate limit on successful login
	if h.RateLimiter != nil {
		h.RateLimiter.ResetAttempts(r.Context(), identifier)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
