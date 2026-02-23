package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

var (
	wsAllowedOriginsOnce sync.Once
	wsAllowedOrigins     map[string]struct{}
)

func loadWSAllowedOrigins() map[string]struct{} {
	wsAllowedOriginsOnce.Do(func() {
		wsAllowedOrigins = map[string]struct{}{
			"http://localhost:5173":              {},
			"http://127.0.0.1:5173":              {},
			"http://localhost:5174":              {},
			"http://localhost:5175":              {},
			"https://relaxation-hub.netlify.app": {},
		}

		if raw := strings.TrimSpace(os.Getenv("WS_ALLOWED_ORIGINS")); raw != "" {
			for _, origin := range strings.Split(raw, ",") {
				origin = strings.TrimSpace(origin)
				if origin != "" {
					wsAllowedOrigins[origin] = struct{}{}
				}
			}
		}
	})
	return wsAllowedOrigins
}

func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Native/mobile clients usually have no Origin header.
		return true
	}

	// Allow same-origin upgrades.
	if strings.Contains(origin, "://"+r.Host) {
		return true
	}

	_, ok := loadWSAllowedOrigins()[origin]
	return ok
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return isAllowedWebSocketOrigin(r)
	},
}

// WebSocketHandler handles WebSocket connections and can validate tokens on the
// upgrade request (supports ?token=... for browser clients).
type WebSocketHandler struct {
	hub    *ws.Hub
	jwtKey string
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, jwtKey string) *WebSocketHandler {
	return &WebSocketHandler{hub: hub, jwtKey: jwtKey}
}

// parseTokenFromRequest accepts either Authorization header or ?token=... query param
// and returns the user id if valid.
func (h *WebSocketHandler) parseTokenFromRequest(r *http.Request) (int64, error) {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	var tokenString string
	if authHeader != "" {
		parts := strings.Fields(authHeader)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenString = parts[1]
		}
	}
	// Fallback to ?token= query param
	if tokenString == "" {
		tokenString = r.URL.Query().Get("token")
	}

	if tokenString == "" {
		return 0, http.ErrNoCookie
	}

	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtKey), nil
	})
	if err != nil || !token.Valid {
		return 0, err
	}
	return int64(claims.UserID), nil
}

// HandleConnection upgrades HTTP connection to WebSocket after validating JWT
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	slog.Debug("WebSocket: connection attempt", "remote_addr", r.RemoteAddr)

	userID, err := h.parseTokenFromRequest(r)
	if err != nil {
		slog.Warn("WebSocket: token validation failed", "error", err)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slog.Debug("WebSocket: token validated", "user_id", userID)

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}

	slog.Info("WebSocket: connection upgraded", "user_id", userID)

	// Create and register new client
	client := ws.NewClient(h.hub, conn, userID)
	h.hub.Register <- client

	// Start client's read and write pumps
	client.Start()

	slog.Debug("WebSocket: client started", "user_id", userID)
}
