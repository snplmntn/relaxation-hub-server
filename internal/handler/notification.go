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
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    n, err := h.notificationService.Create(r.Context(), &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(toNotificationResponse(n))
}

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    notifs, err := h.notificationService.ListByUser(r.Context(), userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    out := make([]model.NotificationResponse, 0, len(notifs))
    for i := range notifs {
        out = append(out, toNotificationResponse(&notifs[i]))
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(out)
}

func (h *NotificationHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    nid, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        http.Error(w, "invalid notification id", http.StatusBadRequest)
        return
    }

    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    if err := h.notificationService.MarkAsRead(r.Context(), nid, userID); err != nil {
        if err == pgx.ErrNoRows {
            http.Error(w, "notification not found", http.StatusNotFound)
            return
        }
        http.Error(w, err.Error(), http.StatusBadRequest)
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
