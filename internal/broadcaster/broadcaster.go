package broadcaster

import (
	"log"

	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

// This package provides a thin adapter so existing server code can call
// broadcaster.BroadcastToUser(...) while we use the existing gorilla/websocket
// hub implementation.

var hub *ws.Hub

// SetHub wires the websocket Hub created in main into this adapter.
func SetHub(h *ws.Hub) {
    hub = h
}

// BroadcastToUser proxies to the websocket Hub's SendToUser method.
// It is defined as a variable to allow mocking in tests.
var BroadcastToUser = func(userID int64, event string, data interface{}) error {
    if hub == nil {
        log.Printf("broadcaster.BroadcastToUser: Hub is nil! Cannot send event '%s' to user %d", event, userID)
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
