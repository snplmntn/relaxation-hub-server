package broadcaster

import (
	"context"
	"log/slog"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

// This package provides a thin adapter so existing server code can call
// broadcaster.BroadcastToUser(...) while we use the existing gorilla/websocket
// hub implementation.

var hub *ws.Hub
var userRepo repository.UserRepository

// SetHub wires the websocket Hub created in main into this adapter.
func SetHub(h *ws.Hub) {
	hub = h
}

// SetUserRepo wires the user repository so we can fan-out to all admins.
func SetUserRepo(r repository.UserRepository) {
	userRepo = r
}

// BroadcastToUser proxies to the websocket Hub's SendToUser method.
// It is defined as a variable to allow mocking in tests.
var BroadcastToUser = func(userID int64, event string, data interface{}) error {
	if hub == nil {
		slog.Warn("broadcaster.BroadcastToUser: Hub is nil! Cannot send event to user", "event", event, "user_id", userID)
		return nil
	}
	return hub.SendToUser(userID, event, data)
}

// IsUserOnline check if a user is currently connected to the websocket hub.
var IsUserOnline = func(userID int64) bool {
	if hub == nil {
		return false
	}
	return hub.IsUserOnline(userID)
}

var sendToUsers = func(userIDs []int64, event string, data interface{}) error {
	if hub == nil {
		return nil
	}
	return hub.SendToUsers(userIDs, event, data)
}

// BroadcastToAdmins sends an event to every admin user. It is safe to call when the
// hub or userRepo is nil; in those cases it no-ops with a debug log.
func BroadcastToAdmins(ctx context.Context, event string, data interface{}) error {
	if hub == nil {
		slog.Debug("broadcaster.BroadcastToAdmins: hub is nil, skipping broadcast", "event", event)
		return nil
	}
	if userRepo == nil {
		slog.Debug("broadcaster.BroadcastToAdmins: user repo is nil, skipping broadcast", "event", event)
		return nil
	}

	admins, err := userRepo.ListUsers(ctx, "admin")
	if err != nil {
		slog.Warn("broadcaster.BroadcastToAdmins: failed to list admins", "event", event, "error", err)
		return err
	}

	superAdmins, err := userRepo.ListUsers(ctx, "super_admin")
	if err != nil {
		slog.Warn("broadcaster.BroadcastToAdmins: failed to list super admins", "event", event, "error", err)
		superAdmins = nil
	}

	if len(admins) == 0 && len(superAdmins) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(admins)+len(superAdmins))
	userIDs := make([]int64, 0, len(admins)+len(superAdmins))
	appendUnique := func(users []model.User) {
		for _, u := range users {
			userID := int64(u.UserID)
			if _, exists := seen[userID]; exists {
				continue
			}
			seen[userID] = struct{}{}
			userIDs = append(userIDs, userID)
		}
	}
	appendUnique(admins)
	appendUnique(superAdmins)

	return sendToUsers(userIDs, event, data)
}
