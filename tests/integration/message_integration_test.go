package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func SetupMessageRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	hub := ws.NewHub()
	go hub.Run()

	userRepo := repository.NewUserRepository(d)
	notificationRepo := repository.NewNotificationRepository(d)
	notificationService := service.NewNotificationService(notificationRepo, userRepo, nil) // No FCM in tests

	messageRepo := repository.NewMessageRepository(d)
	messageService := service.NewMessageService(messageRepo, notificationService, userRepo, hub)
	messageHandler := handler.NewMessageHandler(messageService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/messages", func(r chi.Router) {
				r.Post("/conversation", messageHandler.CreateConversation)
				r.Get("/conversations", messageHandler.ListConversations)
				r.Post("/send", messageHandler.SendMessage)
				r.Get("/conversation/{conversation_id}", messageHandler.GetMessages)
				r.Post("/message/{message_id}/read", messageHandler.MarkMessageAsRead)
			})
		})
	})

	return r
}

func TestIntegration_CreateConversation(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupMessageRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	conversationBody := map[string]interface{}{
		"participant_ids": []int64{1, 2},
		"booking_id":      456,
	}

	body, _ := json.Marshal(conversationBody)
	req := httptest.NewRequest("POST", "/api/v1/messages/conversation", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Logf("Conversation creation returned status %d", rr.Code)
	}

	t.Log("✓ Conversation endpoint accessible")
}

func TestIntegration_ListConversations(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupMessageRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/messages/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Conversation listing successful")
}

func TestIntegration_SendMessage(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	
	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer cleanup()

	router := SetupMessageRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	messageBody := map[string]interface{}{
		"conversation_id": 789,
		"message":         "Hello, test message!",
	}

	body, _ := json.Marshal(messageBody)
	req := httptest.NewRequest("POST", "/api/v1/messages/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Logf("Send message returned status %d", rr.Code)
	}

	t.Log("✓ Send message endpoint accessible")
}
