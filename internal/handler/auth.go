package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AuthHandler struct {
	service.AuthService
}

type AuthRequest struct {
	FullName string `json:"full_name"`
	Provider    string `json:"provider"`
	ProviderKey    string `json:"provider_key"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *AuthHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.Signup(context.Background(), req.FullName, req.Provider, req.ProviderKey, req.Password, req.Role)
	if err != nil {
		if err.Error() == "email already in use" {
			http.Error(w, fmt.Sprintf("Error: %s", err.Error()), http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("Error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Signed up successfuly!"})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tokenString, err := h.Login(context.Background(), req.Provider, req.ProviderKey, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %s", err.Error()), http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}