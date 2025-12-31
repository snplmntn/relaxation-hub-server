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
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func SetupAdminRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	adminActionRepo := repository.NewAdminActionRepository(d)
	adminActionService := service.NewAdminActionService(adminActionRepo)
	adminActionHandler := handler.NewAdminActionHandler(adminActionService)

	promotionRepo := repository.NewPromotionRepository(d)
	promotionService := service.NewPromotionService(promotionRepo)
	promotionHandler := handler.NewPromotionHandler(promotionService)

	branchRepo := repository.NewBranchRepository(d)
	branchService := service.NewBranchService(branchRepo)
	branchHandler := handler.NewBranchHandler(branchService)

	notificationRepo := repository.NewNotificationRepository(d)
	notificationService := service.NewNotificationService(notificationRepo)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Get("/promotions", promotionHandler.ListActivePromotions)
			r.Get("/branches", branchHandler.ListBranches)
			r.Get("/branches/{id}", branchHandler.GetBranch)
			r.Get("/notifications", notificationHandler.ListNotifications)
			r.Post("/notifications/{id}/read", notificationHandler.MarkNotificationAsRead)

			r.Route("/admin", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})

				r.Post("/actions", adminActionHandler.LogAction)
				r.Get("/actions", adminActionHandler.GetAllActions)
				r.Get("/actions/me", adminActionHandler.GetMyActions)
			})

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/promotions", promotionHandler.CreatePromotion)

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/branches", branchHandler.CreateBranch)

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Patch("/branches/{id}", branchHandler.UpdateBranch)

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/notifications", notificationHandler.CreateNotification)
		})
	})

	return r
}

func TestIntegration_ListPromotions(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/promotions", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Promotion listing successful")
}

func TestIntegration_CreatePromotion_AdminOnly(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	adminToken := createTestUser(t, tx, "admin@test.com", "admin")

	promotionBody := map[string]interface{}{
		"code":                "TEST20",
		"description":         "Test promotion",
		"discount_percentage": 20,
		"max_uses":            100,
	}

	body, _ := json.Marshal(promotionBody)
	req := httptest.NewRequest("POST", "/api/v1/promotions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Promotion creation successful")
}

func TestIntegration_ListBranches(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/branches", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Branch listing successful")
}

func TestIntegration_CreateBranch_AdminOnly(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	adminToken := createTestUser(t, tx, "admin@test.com", "admin")

	branchBody := map[string]interface{}{
		"name":    "Test Branch",
		"address": "Test Address",
		"phone":   "+1234567890",
		"email":   "branch@test.com",
	}

	body, _ := json.Marshal(branchBody)
	req := httptest.NewRequest("POST", "/api/v1/branches", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Branch creation successful")
}

func TestIntegration_AdminActions(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	adminToken := createTestUser(t, tx, "admin@test.com", "admin")

	actionBody := map[string]interface{}{
		"action_type": "test_action",
		"target_type": "user",
		"target_id":   123,
		"description": "Test action",
	}

	body, _ := json.Marshal(actionBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/actions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Admin action logging successful")
}

func TestIntegration_ListNotifications(t *testing.T) {
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

	router := SetupAdminRouter(tx, getTestConfig())
	token := createTestUser(t, tx, "user@test.com", "client")

	req := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	t.Log("✓ Notification listing successful")
}
