package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type NotificationHandler struct {
	notificationService *service.NotificationService
}

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

func (h *NotificationHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req model.CreateNotificationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	n, err := h.notificationService.Create(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toNotificationResponse(n))
}

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// Parse pagination parameters (default: page=1, limit=50)
	page := 1
	limit := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	paginatedResp, err := h.notificationService.ListByUser(r.Context(), userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paginatedResp)
}

func (h *NotificationHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	nid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.notificationService.MarkAsRead(r.Context(), nid, userID); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "notification not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toNotificationResponse(n *model.Notification) model.NotificationResponse {
	return model.NotificationResponse{
		NotificationID: n.NotificationID,
		Type:           n.Type,
		Title:          n.Title,
		Message:        n.Message,
		IsRead:         n.IsRead,
		ReadAt:         n.ReadAt,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}
