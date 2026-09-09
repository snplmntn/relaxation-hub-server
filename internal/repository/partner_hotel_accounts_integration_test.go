package repository_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/require"
)

// All DDL and data live in a unique schema within a transaction that is rolled back.
// No existing application tables are read or modified.
func TestPartnerHotelAccountsIntegration(t *testing.T) {
	url := os.Getenv("HOTEL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set HOTEL_TEST_DATABASE_URL for PostgreSQL integration coverage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	schema := fmt.Sprintf("hotel_access_test_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema)
	require.NoError(t, err)
	initial, err := os.ReadFile("../db/migrations/001_init.sql")
	require.NoError(t, err)
	core := string(initial)
	core = core[strings.Index(core, "CREATE TABLE IF NOT EXISTS users ("):strings.Index(core, "CREATE TABLE IF NOT EXISTS addresses (")]
	_, err = tx.Exec(ctx, core)
	require.NoError(t, err)
	for _, file := range []string{"026_add_unique_primary_phone_index.sql", "037_create_partner_hotels.sql", "038_partner_hotel_accounts.sql"} {
		sql, err := os.ReadFile("../db/migrations/" + file)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, strings.ReplaceAll(string(sql), "public.", schema+"."))
		require.NoError(t, err)
	}
	repo := repository.NewPartnerHotelRepository(tx)
	svc := service.NewPartnerHotelService(repo)
	userRepo := repository.NewUserRepository(tx)
	authSvc := service.NewAuthService(userRepo, &config.Config{JWTKey: "hotel-integration-test-key"}, repo)
	hotel, err := svc.CreateHotel(ctx, &model.CreatePartnerHotelRequest{HotelName: "First hotel"})
	require.NoError(t, err)
	other, err := svc.CreateHotel(ctx, &model.CreatePartnerHotelRequest{HotelName: "Second hotel"})
	require.NoError(t, err)
	create := func(hotelID int64, email, role string) *model.PartnerHotelStaff {
		staff, err := svc.CreateStaff(ctx, hotelID, &model.CreatePartnerHotelStaffRequest{
			FullName: "Hotel Test", Email: email, AccessRole: role, Password: "TestingPass1!",
		})
		require.NoError(t, err)
		require.NotNil(t, staff.UserID)
		return staff
	}
	admin := create(hotel.PartnerHotelID, "admin@example.test", model.RoleHotelAdmin)
	create(hotel.PartnerHotelID, "admin2@example.test", model.RoleHotelAdmin)
	member := create(hotel.PartnerHotelID, "staff@example.test", model.RoleHotelStaff)
	outsider := create(other.PartnerHotelID, "other@example.test", model.RoleHotelAdmin)
	checkHotelDayViewPrivacy(t, ctx, tx, svc, admin, member)
	checkHotelLoginErrorClassification(t, ctx, userRepo, repo, admin.Email)
	for _, account := range []*model.PartnerHotelStaff{admin, member, outsider} {
		token, err := authSvc.Login(ctx, "email", account.Email, "TestingPass1!")
		require.NoError(t, err)
		require.NotEmpty(t, token)
	}
	_, _, err = authSvc.Signup(ctx, "email", "escalation@example.test", "TestingPass1!", model.RoleHotelAdmin)
	require.Error(t, err, "public signup cannot grant hotel roles")
	_, err = svc.CreateStaff(ctx, hotel.PartnerHotelID, &model.CreatePartnerHotelStaffRequest{FullName: "Invalid", Email: "invalid@example.test", Password: "TestingPass1!", AccessRole: model.RoleSuperAdmin})
	require.Error(t, err, "hotel staff cannot become platform administrators")

	directory, err := svc.ListMyHotelStaff(ctx, *admin.UserID)
	require.NoError(t, err)
	require.Len(t, directory, 3, "multiple admins belong to one hotel")
	_, err = svc.ListMyHotelStaff(ctx, *member.UserID)
	require.ErrorIs(t, err, service.ErrHotelAccessDenied)
	directory, err = svc.ListMyHotelStaff(ctx, *outsider.UserID)
	require.NoError(t, err)
	require.Len(t, directory, 1, "hotel directory is scoped to the signed-in user")
	_, err = svc.GetAccess(ctx, 999999)
	require.ErrorIs(t, err, service.ErrHotelAccessDenied)

	before := 0
	require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&before))
	_, err = svc.CreateStaff(ctx, other.PartnerHotelID, &model.CreatePartnerHotelStaffRequest{FullName: "Duplicate", Email: admin.Email, Password: "TestingPass1!", AccessRole: model.RoleHotelAdmin})
	require.ErrorContains(t, err, "already in use")
	after := 0
	require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&after))
	require.Equal(t, before, after, "failed provisioning leaves no orphan account")
	phone := "+639171234567"
	_, err = svc.UpdateStaff(ctx, hotel.PartnerHotelID, admin.PartnerHotelStaffID, &model.UpdatePartnerHotelStaffRequest{Phone: &phone})
	require.NoError(t, err)
	_, err = svc.CreateStaff(ctx, other.PartnerHotelID, &model.CreatePartnerHotelStaffRequest{FullName: "Duplicate phone", Email: "unique-email@example.test", Phone: phone, Password: "TestingPass1!", AccessRole: model.RoleHotelStaff})
	require.ErrorContains(t, err, "phone number is already in use")
	require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&after))
	require.Equal(t, before, after)

	h := handler.NewPartnerHotelHandler(svc)
	request := func(path string, account *model.PartnerHotelStaff, jwtRole string, endpoint http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		token, err := auth.GenerateToken(int(*account.UserID), jwtRole, "hotel-integration-test-key")
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		middleware.AuthMiddleware(h.RestrictHotelAccount(endpoint), "hotel-integration-test-key").ServeHTTP(rr, req)
		return rr
	}
	require.Equal(t, http.StatusOK, request("/api/v1/hotel/access", member, model.RoleHotelStaff, h.MyAccess).Code)
	require.Equal(t, http.StatusForbidden, request("/api/v1/hotel/staff", member, model.RoleHotelStaff, h.MyStaff).Code)
	require.Equal(t, http.StatusForbidden, request("/api/v1/partner-hotels", admin, model.RoleHotelAdmin, h.ListHotels).Code)
	require.NotContains(t, request("/api/v1/hotel/staff", admin, model.RoleHotelAdmin, h.MyStaff).Body.String(), "password")

	demoted := model.RoleHotelStaff
	_, err = svc.UpdateStaff(ctx, hotel.PartnerHotelID, admin.PartnerHotelStaffID, &model.UpdatePartnerHotelStaffRequest{AccessRole: &demoted})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, request("/api/v1/hotel/staff", admin, model.RoleHotelAdmin, h.MyStaff).Code, "old admin JWT cannot bypass demotion")

	inactive := false
	_, err = svc.UpdateStaff(ctx, hotel.PartnerHotelID, member.PartnerHotelStaffID, &model.UpdatePartnerHotelStaffRequest{IsActive: &inactive})
	require.NoError(t, err)
	_, err = authSvc.Login(ctx, "email", member.Email, "TestingPass1!")
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, request("/api/v1/hotel/access", member, model.RoleHotelStaff, h.MyAccess).Code)
	_, err = svc.UpdateHotel(ctx, other.PartnerHotelID, &model.UpdatePartnerHotelRequest{IsActive: &inactive})
	require.NoError(t, err)
	_, err = authSvc.Login(ctx, "email", outsider.Email, "TestingPass1!")
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, request("/api/v1/hotel/access", outsider, model.RoleHotelAdmin, h.MyAccess).Code)

	email := "updated@example.test"
	password := "UpdatedPass2!"
	_, err = svc.UpdateStaff(ctx, hotel.PartnerHotelID, admin.PartnerHotelStaffID, &model.UpdatePartnerHotelStaffRequest{Email: &email, Password: &password})
	require.NoError(t, err)
	_, err = authSvc.Login(ctx, "email", admin.Email, "TestingPass1!")
	require.Error(t, err)
	_, err = authSvc.Login(ctx, "email", email, password)
	require.NoError(t, err)

	var legacyID int64
	require.NoError(t, tx.QueryRow(ctx, `INSERT INTO partner_hotel_staff(partner_hotel_id,full_name,email) VALUES($1,'Legacy contact','legacy@example.test') RETURNING partner_hotel_staff_id`, hotel.PartnerHotelID).Scan(&legacyID))
	enabled, err := svc.UpdateStaff(ctx, hotel.PartnerHotelID, legacyID, &model.UpdatePartnerHotelStaffRequest{Password: &password})
	require.NoError(t, err)
	require.NotNil(t, enabled.UserID)
	_, err = authSvc.Login(ctx, "email", "legacy@example.test", password)
	require.NoError(t, err)
	runHotelBrowserQA(t, ctx, tx)
}
