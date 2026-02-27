package broadcaster

import (
	"context"
	"log/slog"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

// This package provides a thin adapter so existing server code can call
// broadcaster.BroadcastToUser(...) while we use the existing gorilla/websocket
// hub implementation.

var hub *ws.Hub
var userRepo repository.UserRepository

func normalizeEventName(event string) string {
	normalized := strings.TrimSpace(event)
	switch normalized {
	case "":
		return normalized
	case "booking:new", "booking:created":
		return "booking.created"
	case "booking:updated", "booking_update":
		return "booking.updated"
	case "booking:assigned":
		return "booking.assigned"
	case "message:new":
		return "message.created"
	case "notification", "notification:created":
		return "notification.created"
	case "offered_to_therapist", "new_booking_offer":
		return "booking.offer.created"
	case "offer_accepted":
		return "booking.offer.accepted"
	case "offer_expired":
		return "booking.offer.expired"
	case "offer_cancelled":
		return "booking.offer.cancelled"
	case "offer_declined":
		return "booking.offer.declined"
	case "ride_offer":
		return "ride.offer.created"
	case "ride:accepted":
		return "ride.accepted"
	case "ride:assigned":
		return "ride.assigned"
	case "ride:updated":
		return "ride.updated"
	case "ride:status_updated":
		return "ride.status.updated"
	case "ride:location_update":
		return "ride.location.updated"
	case "ride:cancelled":
		return "ride.cancelled"
	case "ride:unassigned":
		return "ride.unassigned"
	case "extension:requested":
		return "booking.extension.requested"
	case "extension:accepted":
		return "booking.extension.accepted"
	case "extension:rejected":
		return "booking.extension.rejected"
	case "day_view:therapist_order_updated":
		return "day_view.therapist_order_updated"
	default:
		return normalized
	}
}

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
	return hub.SendToUser(userID, normalizeEventName(event), data)
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
	return hub.SendToUsers(userIDs, normalizeEventName(event), data)
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
