package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AccountSecurityHandler struct {
	service *service.AccountSecurityService
}

func NewAccountSecurityHandler(service *service.AccountSecurityService) *AccountSecurityHandler {
	return &AccountSecurityHandler{service: service}
}

func (h *AccountSecurityHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		h.respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password updated."})
}

func (h *AccountSecurityHandler) ResetStaffPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil || userID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.ResetStaffPassword(r.Context(), userID, req.NewPassword); err != nil {
		h.respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Staff password updated."})
}

func (h *AccountSecurityHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.DeleteAccount(r.Context(), userID, req.CurrentPassword); err != nil {
		h.respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AccountSecurityHandler) respondError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		respondServiceError(w, http.StatusBadRequest, validationErr)
		return
	}
	respondError(w, http.StatusInternalServerError, "account security request failed")
}
