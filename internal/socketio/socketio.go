package socketio

import (
	"log"

	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

// This package provides a thin adapter so existing server code can call
// socketio.BroadcastToUser(...) while we use the existing gorilla/websocket
// hub implementation. The old go-socket.io server was removed due to
// engine.io v4 incompatibility with the JS client.

var hub *ws.Hub

// SetHub wires the websocket Hub created in main into this adapter.
func SetHub(h *ws.Hub) {
    hub = h
}

// BroadcastToUser proxies to the websocket Hub's SendToUser method.
// It is defined as a variable to allow mocking in tests.
var BroadcastToUser = func(userID int64, event string, data interface{}) error {
    if hub == nil {
        log.Printf("socketio.BroadcastToUser: Hub is nil! Cannot send event '%s' to user %d", event, userID)
        return nil
    }
    return hub.SendToUser(userID, event, data)
}
