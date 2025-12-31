package websocket

import (
	"encoding/json"
	"log"
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
			log.Printf("Client connected: user_id=%d", client.UserID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if activeClient, ok := h.clients[client.UserID]; ok && activeClient == client {
				delete(h.clients, client.UserID)
				close(client.send)
				log.Printf("Client disconnected: user_id=%d", client.UserID)
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
		log.Printf("WebSocket: User %d not connected, skipping message type=%s", userID, messageType)
		return nil // User not connected, skip
	}

	msg := Message{
		Type: messageType,
		Data: data,
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WebSocket: Failed to marshal message for user %d: %v", userID, err)
		return err
	}

	log.Printf("WebSocket: Sending message type=%s to user=%d, payload_size=%d bytes", messageType, userID, len(jsonMsg))

	select {
	case client.send <- jsonMsg:
		log.Printf("WebSocket: Successfully queued message for user=%d", userID)
	default:
		// Client's send channel is full, close it
		log.Printf("WebSocket: Send channel full for user=%d, closing connection", userID)
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
