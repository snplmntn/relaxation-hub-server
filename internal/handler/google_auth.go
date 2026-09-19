package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type GoogleAuthHandler struct {
	service service.GoogleAuthService
}

func NewGoogleAuthHandler(authService service.GoogleAuthService) *GoogleAuthHandler {
	return &GoogleAuthHandler{service: authService}
}

type googleCredentialRequest struct {
	Credential string `json:"credential"`
}

func (h *GoogleAuthHandler) Authenticate(w http.ResponseWriter, r *http.Request) {
	var req googleCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Credential) == "" {
		respondGoogleAuthError(w, http.StatusBadRequest, "invalid_request", "Google credential is required")
		return
	}

	result, err := h.service.Authenticate(r.Context(), req.Credential)
	if err != nil {
		handleGoogleAuthServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *GoogleAuthHandler) Link(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondGoogleAuthError(w, http.StatusUnauthorized, "unauthorized", "Authenticated user not found")
		return
	}
	var req googleCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Credential) == "" {
		respondGoogleAuthError(w, http.StatusBadRequest, "invalid_request", "Google credential is required")
		return
	}
	if err := h.service.Link(r.Context(), int(userID), req.Credential); err != nil {
		handleGoogleAuthServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"connected": true})
}

func handleGoogleAuthServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, oauth.ErrGoogleCredentialNotConfigured):
		respondGoogleAuthError(w, http.StatusServiceUnavailable, "google_not_configured", "Google sign-in is not configured")
	case errors.Is(err, service.ErrInvalidGoogleCredential), errors.Is(err, service.ErrGoogleEmailUnverified):
		respondGoogleAuthError(w, http.StatusUnauthorized, "invalid_google_credential", "Google could not verify this account")
	case errors.Is(err, service.ErrGoogleAccountLinkNeeded):
		respondGoogleAuthError(w, http.StatusConflict, "account_link_required", "An account already exists for this email. Sign in with your password, then connect Google from your profile.")
	case errors.Is(err, service.ErrGoogleIdentityInUse):
		respondGoogleAuthError(w, http.StatusConflict, "google_identity_in_use", "This Google account is already connected to another user")
	case strings.HasPrefix(err.Error(), "Account is "):
		respondGoogleAuthError(w, http.StatusForbidden, "account_restricted", err.Error())
	default:
		respondGoogleAuthError(w, http.StatusInternalServerError, "google_auth_failed", "Google sign-in could not be completed")
	}
}

func respondGoogleAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"code": code, "error": message, "message": message})
}
