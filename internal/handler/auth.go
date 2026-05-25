package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AuthHandler struct {
	AuthService        service.AuthService
	RateLimiter        *middleware.RateLimiter
	ReferralService    service.ReferralService
	RideRepository     repository.RideRepository // For creating rider profiles
	RiderWalletService *service.RiderWalletService
}

func NewAuthHandler(
	authService service.AuthService,
	rateLimiter *middleware.RateLimiter,
	referralService service.ReferralService,
) *AuthHandler {
	return &AuthHandler{
		AuthService:        authService,
		RateLimiter:        rateLimiter,
		ReferralService:    referralService,
		RideRepository:     nil, // Will be set via SetRideRepository
		RiderWalletService: nil,
	}
}

// SetRideRepository allows setting the ride repository after initialization
func (h *AuthHandler) SetRideRepository(repo repository.RideRepository) {
	h.RideRepository = repo
}

// SetRiderWalletService allows setting the rider wallet service after initialization
func (h *AuthHandler) SetRiderWalletService(s *service.RiderWalletService) {
	h.RiderWalletService = s
}

type AuthRequest struct {
	Provider     string `json:"provider"`
	ProviderKey  string `json:"provider_key"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	ReferralCode string `json:"referral_code,omitempty"` // For signup with referral
	// Rider-specific fields
	FullName     string `json:"full_name,omitempty"`     // For rider registration
	Phone        string `json:"phone,omitempty"`         // For rider registration
	VehicleType  string `json:"vehicle_type,omitempty"`  // For rider: motorcycle, car, suv
	LicensePlate string `json:"license_plate,omitempty"` // For rider
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
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, token, err := h.AuthService.Signup(r.Context(), req.Provider, req.ProviderKey, req.Password, req.Role)
	if err != nil {
		if err.Error() == "email already in use" {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
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

	// Create rider profile if role is rider
	if req.Role == "rider" && h.RideRepository != nil {
		vehicleType := req.VehicleType
		if vehicleType == "" {
			vehicleType = "motorcycle" // Default to motorcycle
		}
		err = h.RideRepository.CreateRiderProfile(r.Context(), int64(userID), vehicleType, req.LicensePlate)
		if err != nil {
			// Log error but don't fail signup - rider can update profile later
			fmt.Printf("Warning: Failed to create rider profile for user %d: %v\n", userID, err)
		} else if h.RiderWalletService != nil {
			// After creating the profile, create the initial wallet and performance records.
			err = h.RiderWalletService.CreateInitialRiderRecords(r.Context(), int64(userID))
			if err != nil {
				// Log this error as well, but don't fail the signup.
				// The user exists, they can contact support if wallet is missing.
				fmt.Printf("Warning: Failed to create initial wallet/performance records for user %d: %v\n", userID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// For client signups return a token immediately; otherwise return a message
	if token != "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":   token,
			"user_id": userID,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Signed up successfully!",
		"user_id": userID,
	})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
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
			waitMinutes := int(time.Until(lockedUntil).Minutes()) + 1
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(lockedUntil).Seconds())))
			respondError(w, http.StatusTooManyRequests, fmt.Sprintf("Too many failed attempts. Account locked for %d minutes.", waitMinutes))
			return
		}
	}

	tokenString, err := h.AuthService.Login(r.Context(), req.Provider, req.ProviderKey, req.Password)
	if err != nil {
		errMsg := err.Error()
		if isLoginBackendError(errMsg) {
			slog.Warn("login backend failure", "provider", req.Provider, "identifier", identifier, "error", err)
			respondError(w, http.StatusServiceUnavailable, "Authentication service unavailable")
			return
		}

		if isAccountStatusError(errMsg) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   errMsg,
				"message": errMsg,
			})
			return
		}

		if h.RateLimiter != nil {
			h.RateLimiter.RecordFailedAttempt(r.Context(), identifier)
		}

		respondError(w, http.StatusUnauthorized, err.Error())
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

func isLoginBackendError(errMsg string) bool {
	normalized := strings.ToLower(errMsg)
	return strings.Contains(normalized, "login identity lookup failed") ||
		strings.Contains(normalized, "login user lookup failed")
}

func isAccountStatusError(errMsg string) bool {
	normalized := strings.ToLower(errMsg)
	return strings.Contains(normalized, "banned") ||
		strings.Contains(normalized, "suspended") ||
		strings.Contains(normalized, "inactive") ||
		strings.Contains(normalized, "blocked") ||
		strings.Contains(normalized, "not active")
}
