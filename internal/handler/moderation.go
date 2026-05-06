package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ModerationHandler struct {
	moderationService service.ModerationService
}

func NewModerationHandler(moderationService service.ModerationService) *ModerationHandler {
	return &ModerationHandler{moderationService: moderationService}
}

type BlockUserRequest struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason,omitempty"`
}

func (h *ModerationHandler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	entries, err := h.moderationService.ListBlockedUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list moderation blocks")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"blocked_users": entries,
		"count":         len(entries),
	})
}

func (h *ModerationHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req BlockUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)

	if err := h.moderationService.BlockUser(r.Context(), adminID, req.UserID, req.Reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

func (h *ModerationHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid user id in path")
		return
	}

	if err := h.moderationService.UnblockUser(r.Context(), userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}
