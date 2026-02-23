package broadcaster

import (
	"context"
	"log/slog"

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
	if len(admins) == 0 {
		return nil
	}

	userIDs := make([]int64, 0, len(admins))
	for _, admin := range admins {
		userIDs = append(userIDs, int64(admin.UserID))
	}

	return hub.SendToUsers(userIDs, event, data)
}
