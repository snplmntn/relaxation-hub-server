package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// LandingSettingsHandler handles the public landing page settings endpoints.
type LandingSettingsHandler struct {
	settings *service.LandingSettingsService
}

// NewLandingSettingsHandler creates a new LandingSettingsHandler.
func NewLandingSettingsHandler(settings *service.LandingSettingsService) *LandingSettingsHandler {
	return &LandingSettingsHandler{settings: settings}
}

// GetLandingSettings handles GET /landing-settings (public).
func (h *LandingSettingsHandler) GetLandingSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Get(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load landing settings")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	respondJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
}

// UpdateLandingSettings handles PATCH /landing-settings (super admin only).
func (h *LandingSettingsHandler) UpdateLandingSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.settings.Update(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update landing settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
}
