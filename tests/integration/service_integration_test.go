package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func SetupServiceRouter(pool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	serviceRepo := repository.NewServiceRepository(pool)
	serviceCatalog := service.NewServiceCatalog(serviceRepo)
	serviceHandler := handler.NewServiceHandler(serviceCatalog)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/services", serviceHandler.ListServices)

		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})
			r.Use(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			})
			r.Post("/services", serviceHandler.CreateService)
		})
	})

	return r
}

func TestIntegration_ListServices(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	router := SetupServiceRouter(pool, getTestConfig())

	req := httptest.NewRequest("GET", "/api/v1/services", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["services"] == nil {
		t.Fatal("Expected services array in response")
	}

	t.Log("✓ Service listing successful")
}

func TestIntegration_CreateService_AdminOnly(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupServiceRouter(pool, getTestConfig())

	adminToken := createTestUser(t, pool, "admin@test.com", "admin")

	serviceBody := map[string]interface{}{
		"name":             "Test Massage",
		"description":      "Test description",
		"base_price":       1500.0,
		"duration_minutes": 60,
		"category":         "massage",
	}

	body, _ := json.Marshal(serviceBody)
	req := httptest.NewRequest("POST", "/api/v1/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Service creation successful")
}

func TestIntegration_CreateService_ClientForbidden(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	router := SetupServiceRouter(pool, getTestConfig())

	clientToken := createTestUser(t, pool, "client@test.com", "client")

	serviceBody := map[string]interface{}{
		"name":             "Test Massage",
		"description":      "Test description",
		"base_price":       1500.0,
		"duration_minutes": 60,
		"category":         "massage",
	}

	body, _ := json.Marshal(serviceBody)
	req := httptest.NewRequest("POST", "/api/v1/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+clientToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}

	t.Log("✓ Non-admin correctly forbidden from creating services")
}
