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
				r.Get("/summary", referralHandler.GetReferralSummary)
				r.Get("/my-code", referralHandler.GetMyReferralCode)
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
	})
}

func TestIntegration_GetReferralSummary(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		router := SetupReferralRouter(tx, getTestConfig())
		token, _, _ := createTestUser(t, tx, "ref_summary_user", "client")
		_, referredUserOneID, _ := createTestUser(t, tx, "ref_summary_friend_1", "client")
		_, referredUserTwoID, _ := createTestUser(t, tx, "ref_summary_friend_2", "client")

		createReferral := func(referredID int64) int64 {
			body, _ := json.Marshal(map[string]interface{}{
				"referred_id": referredID,
			})
			req := httptest.NewRequest("POST", "/api/v1/referrals", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusCreated {
				t.Fatalf("expected referral create status 201, got %d body=%s", rr.Code, rr.Body.String())
			}

			var created map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
				t.Fatalf("failed to decode create referral response: %v", err)
			}

			referralIDFloat, ok := created["referral_id"].(float64)
			if !ok {
				t.Fatalf("expected referral_id in create response, got: %v", created)
			}
			return int64(referralIDFloat)
		}

		completedReferralID := createReferral(referredUserOneID)
		_ = createReferral(referredUserTwoID)

		if _, err := tx.Exec(
			t.Context(),
			"UPDATE referrals SET status = 'completed', reward_earned = true, completed_at = now() WHERE referral_id = $1",
			completedReferralID,
		); err != nil {
			t.Fatalf("failed to set referral as completed: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/v1/referrals/summary", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
		}

		var summary map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &summary); err != nil {
			t.Fatalf("failed to decode summary response: %v", err)
		}

		if summary["total_referrals"] != float64(2) {
			t.Fatalf("expected total_referrals=2, got %v", summary["total_referrals"])
		}
		if summary["successful_referrals"] != float64(1) {
			t.Fatalf("expected successful_referrals=1, got %v", summary["successful_referrals"])
		}
		if summary["pending_referrals"] != float64(1) {
			t.Fatalf("expected pending_referrals=1, got %v", summary["pending_referrals"])
		}
		if _, ok := summary["rewards"]; !ok {
			t.Fatalf("expected rewards key in summary response, got %v", summary)
		}
	})
}

func TestIntegration_GetMyReferralCode(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		router := SetupReferralRouter(tx, getTestConfig())
		token, _, _ := createTestUser(t, tx, "my_ref_code_user", "client")
		_, referredUserID, _ := createTestUser(t, tx, "my_ref_code_friend", "client")

		body, _ := json.Marshal(map[string]interface{}{
			"referred_id": referredUserID,
		})
		createReq := httptest.NewRequest("POST", "/api/v1/referrals", bytes.NewBuffer(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Authorization", "Bearer "+token)
		createRR := httptest.NewRecorder()
		router.ServeHTTP(createRR, createReq)

		if createRR.Code != http.StatusCreated {
			t.Fatalf("expected referral create status 201, got %d body=%s", createRR.Code, createRR.Body.String())
		}

		var created map[string]interface{}
		if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
			t.Fatalf("failed to decode create referral response: %v", err)
		}

		expectedCode, ok := created["referral_code"].(string)
		if !ok || expectedCode == "" {
			t.Fatalf("expected referral_code in create response, got: %v", created)
		}

		req := httptest.NewRequest("GET", "/api/v1/referrals/my-code", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to decode my-code response: %v", err)
		}

		code, ok := response["referral_code"].(string)
		if !ok || code == "" {
			t.Fatalf("expected non-empty referral_code in response, got: %v", response)
		}
		if code != expectedCode {
			t.Fatalf("expected referral_code=%s, got %s", expectedCode, code)
		}
	})
}
