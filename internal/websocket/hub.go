package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	// Registered clients by user ID
	clients map[int64]*Client

	// Broadcast messages to all clients
	broadcast chan []byte

	// Register requests from clients
	Register chan *Client

	// Unregister requests from clients
	Unregister chan *Client

	mu sync.RWMutex

	// Database pool for enrichment queries
	db *pgxpool.Pool
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int64]*Client),
		broadcast:  make(chan []byte, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// SetPool sets the database pool for the hub (used for user info enrichment)
func (h *Hub) SetPool(db *pgxpool.Pool) {
	h.db = db
}

// Pool returns the database pool
func (h *Hub) Pool() *pgxpool.Pool {
	return h.db
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			// Unregister old connection if exists
			if oldClient, exists := h.clients[client.UserID]; exists {
				close(oldClient.send)
			}
			h.clients[client.UserID] = client
			h.mu.Unlock()
			slog.Debug("client connected", "user_id", client.UserID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if activeClient, ok := h.clients[client.UserID]; ok && activeClient == client {
				delete(h.clients, client.UserID)
				close(client.send)
				slog.Debug("client disconnected", "user_id", client.UserID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client.UserID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID int64, messageType string, data interface{}) error {
	h.mu.RLock()
	client, exists := h.clients[userID]
	h.mu.RUnlock()

	if !exists {
		slog.Debug("WebSocket: user not connected", "user_id", userID, "message_type", messageType)
		return nil // User not connected, skip
	}

	msg := Message{
		Type: messageType,
		Data: data,
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("WebSocket: failed to marshal message", "user_id", userID, "error", err)
		return err
	}

	slog.Debug("WebSocket: sending message", "message_type", messageType, "user_id", userID, "payload_size", len(jsonMsg))

	select {
	case client.send <- jsonMsg:
		slog.Debug("WebSocket: queued message", "user_id", userID)
	default:
		// Client's send channel is full, close it
		slog.Warn("WebSocket: send channel full, closing", "user_id", userID)
		h.mu.Lock()
		close(client.send)
		delete(h.clients, userID)
		h.mu.Unlock()
	}

	return nil
}

// SendToUsers sends a message to multiple users
func (h *Hub) SendToUsers(userIDs []int64, messageType string, data interface{}) error {
	msg := Message{
		Type: messageType,
		Data: data,
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		if client, exists := h.clients[userID]; exists {
			select {
			case client.send <- jsonMsg:
			default:
				// Skip if send buffer is full
			}
		}
	}

	return nil
}

// IsUserOnline checks if a user is connected
func (h *Hub) IsUserOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.clients[userID]
	return exists
}

// GetOnlineUsers returns list of online user IDs
func (h *Hub) GetOnlineUsers() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDs := make([]int64, 0, len(h.clients))
	for userID := range h.clients {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// Message represents a WebSocket message structure
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
