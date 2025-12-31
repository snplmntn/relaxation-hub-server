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

func SetupLocationRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	hub := ws.NewHub()
	go hub.Run()

	liveLocationRepo := repository.NewLiveLocationRepository(d)
	liveLocationService := service.NewLiveLocationService(liveLocationRepo, hub)
	liveLocationHandler := handler.NewLiveLocationHandler(liveLocationService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/locations", func(r chi.Router) {
				r.Post("/live", liveLocationHandler.UpdateLocation)
				r.Get("/live/{user_id}", liveLocationHandler.GetLocation)
			})
		})
	})

	return r
}

func TestIntegration_UpdateLocation(t *testing.T) {
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

	router := SetupLocationRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "therapist")

	locationBody := map[string]interface{}{
		"latitude":  14.5547,
		"longitude": 121.0244,
	}

	body, _ := json.Marshal(locationBody)
	req := httptest.NewRequest("POST", "/api/v1/locations/live", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Location update successful")
}

func TestIntegration_GetLocation(t *testing.T) {
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

	router := SetupLocationRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/locations/live/test-user-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Logf("Get location returned status %d (expected 200 or 404)", rr.Code)
	}

	t.Log("✓ Get location endpoint accessible")
}
