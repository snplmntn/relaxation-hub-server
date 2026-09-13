package repository_test

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/response"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// Opt-in browser adapter: real hotel handlers/services/repositories on the
// integration test's isolated transaction. Unrelated admin-shell feeds are empty.
func runHotelBrowserQA(t *testing.T, ctx context.Context, tx pgx.Tx) {
	script := os.Getenv("HOTEL_BROWSER_QA_SCRIPT")
	if script == "" {
		return
	}
	users := repository.NewUserRepository(tx)
	hotels := repository.NewPartnerHotelRepository(tx)
	authSvc := service.NewAuthService(users, &config.Config{JWTKey: "browser-qa-test-key"}, hotels)
	_, _, err := authSvc.SignupStaff(ctx, "email", "qa-platform@example.test", "TestingPass1!", model.RoleSuperAdmin)
	require.NoError(t, err)
	hotelSvc := service.NewPartnerHotelService(hotels)
	legacyHotel, err := hotelSvc.CreateHotel(ctx, &model.CreatePartnerHotelRequest{HotelName: "Legacy QA Hotel"})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO partner_hotel_staff(partner_hotel_id,full_name) VALUES($1,'Legacy Contact')`, legacyHotel.PartnerHotelID)
	require.NoError(t, err)
	hotelHandler := handler.NewPartnerHotelHandler(hotelSvc)
	dayHandler := handler.NewHotelDayViewHandler(service.NewHotelDayViewService(hotelSvc, repository.NewHotelDayViewRepository(tx), nil))
	r := chi.NewRouter()
	// pgx.Tx owns one connection; browser parallel requests must be serialized.
	var mu sync.Mutex
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); defer mu.Unlock(); next.ServeHTTP(w, r) })
	})
	r.Post("/api/v1/login", handler.NewAuthHandler(authSvc, nil, nil).HandleLogin)
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler { return middleware.AuthMiddleware(next, "browser-qa-test-key") })
		r.Use(middleware.NewAccountStatusMiddleware(users))
		r.Use(hotelHandler.RestrictHotelAccount)
		r.Get("/api/v1/users/profile", func(w http.ResponseWriter, r *http.Request) {
			id, _ := middleware.GetUserID(r)
			user, err := users.FindUserByID(r.Context(), int(id))
			if err != nil {
				response.RespondError(w, 500, "profile unavailable")
				return
			}
			response.RespondJSON(w, 200, user)
		})
		r.Get("/api/v1/hotel/access", hotelHandler.MyAccess)
		r.Get("/api/v1/hotel/staff", hotelHandler.MyStaff)
		r.Get("/api/v1/hotel/day-view", dayHandler.Get)
		r.Route("/api/v1/partner-hotels", func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			})
			r.Get("/", hotelHandler.ListHotels)
			r.Post("/", hotelHandler.CreateHotel)
			r.Patch("/{hotelID}", hotelHandler.UpdateHotel)
			r.Get("/{hotelID}/staff", hotelHandler.ListStaff)
			r.Post("/{hotelID}/staff", hotelHandler.CreateStaff)
			r.Patch("/{hotelID}/staff/{staffID}", hotelHandler.UpdateStaff)
		})
		r.Get("/api/v1/*", func(w http.ResponseWriter, _ *http.Request) { response.RespondJSON(w, 200, []string{}) })
	})
	server := httptest.NewServer(r)
	defer server.Close()
	command := exec.CommandContext(ctx, "node", script)
	command.Env = append(os.Environ(), "HOTEL_QA_API="+server.URL)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	t.Log(string(output))
}
