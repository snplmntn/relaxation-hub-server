package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	testhelpers "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
)

func SetupReferralRouter(d db.DBTX, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	referralRepo := repository.NewReferralRepository(d)
	referralService := service.NewReferralService(referralRepo)
	referralHandler := handler.NewReferralHandler(referralService)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/referrals", func(r chi.Router) {
				r.Post("/", referralHandler.CreateReferral)
				r.Get("/", referralHandler.ListReferrals)
				r.Get("/code", referralHandler.GetReferralByCode)
				r.Get("/rewards", referralHandler.GetRewards)
				r.Post("/rewards/{reward_id}/redeem", referralHandler.RedeemReward)
			})
		})
	})

	return r
}

func TestIntegration_CreateReferral(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	
	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		router := SetupReferralRouter(tx, getTestConfig())
		token, _, _ := createTestUser(t, tx, "user", "client")
		_, refereeID, _ := createTestUser(t, tx, "friend", "client")

		referralBody := map[string]interface{}{
			"referred_id": refereeID,
		}

		body, _ := json.Marshal(referralBody)
		req := httptest.NewRequest("POST", "/api/v1/referrals", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Logf("Response Body: %s", rr.Body.String())
			t.Fatalf("Expected status 201, got %d", rr.Code)
		}

		t.Log("✓ Referral creation successful")
	})
}

func TestIntegration_ListReferrals(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	
	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		router := SetupReferralRouter(tx, getTestConfig())
		token, _, _ := createTestUser(t, tx, "user", "client")

		req := httptest.NewRequest("GET", "/api/v1/referrals", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("Response Body: %s", rr.Body.String())
			t.Fatalf("Expected status 200, got %d", rr.Code)
		}

		t.Log("✓ Referral listing successful")
	})
}

func TestIntegration_GetRewards(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	
	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		router := SetupReferralRouter(tx, getTestConfig())
		token, _, _ := createTestUser(t, tx, "user", "client")

		req := httptest.NewRequest("GET", "/api/v1/referrals/rewards", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("Response Body: %s", rr.Body.String())
			t.Fatalf("Expected status 200, got %d", rr.Code)
		}

		t.Log("✓ Rewards listing successful")
	})
}
