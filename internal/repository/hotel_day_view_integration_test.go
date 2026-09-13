package repository_test

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Called inside the existing isolated-schema, rollback-only hotel integration test.
func checkHotelDayViewPrivacy(t *testing.T, ctx context.Context, tx pgx.Tx, access *service.PartnerHotelService, admin, staff *model.PartnerHotelStaff) {
	_, err := tx.Exec(ctx, `
		ALTER TABLE users ADD COLUMN nickname TEXT;
		CREATE TABLE branches(branch_id INTEGER PRIMARY KEY,branch_name TEXT,is_active BOOLEAN,deleted_at TIMESTAMP);
		CREATE TABLE therapist_profiles(therapist_id INTEGER PRIMARY KEY,branch_id INTEGER,accept_assignments BOOLEAN,deleted_at TIMESTAMP);
		CREATE TABLE bookings(booking_id SERIAL PRIMARY KEY,therapist_id INTEGER,scheduled_start TIMESTAMP,duration_minutes INTEGER,status TEXT,guest_name TEXT,notes TEXT,reference_code TEXT,client_id INTEGER,final_total NUMERIC);
		INSERT INTO branches VALUES(1,'Main Branch',TRUE,NULL),(2,'Inactive branch',FALSE,NULL);
		INSERT INTO users(user_id,full_name,nickname,role,primary_email) VALUES(1001,'Therapist private surname','Janice','therapist','private-therapist@example.test'),(1002,'Offline Therapist',NULL,'therapist',NULL),(1003,'Hidden Therapist',NULL,'therapist',NULL);
		INSERT INTO therapist_profiles VALUES(1001,1,TRUE,NULL),(1002,1,FALSE,NULL),(1003,2,TRUE,NULL);
		INSERT INTO bookings(therapist_id,scheduled_start,duration_minutes,status,guest_name,notes,reference_code,client_id,final_total) VALUES
		(1001,'2026-09-08 04:30',60,'assigned','Secret Guest','Secret Address','PRIVATE-BOOKING',999,2000),
		(1001,'2026-09-08 05:15',45,'completed','Secret Guest','Secret Address','PRIVATE-BOOKING2',999,2000),
		(1001,'2026-09-08 08:00',60,'cancelled','Secret Guest','Secret Address','CANCELLED',999,2000),
		(1001,'2026-09-08 09:00',60,'cancelled_by_client',NULL,NULL,NULL,999,2000),
		(1001,'2026-09-08 10:00',60,'cancelled_by_therapist',NULL,NULL,NULL,999,2000),
		(1001,'2026-09-08 11:00',60,'rescheduled',NULL,NULL,NULL,999,2000),
		(1001,'2026-09-08 17:00',60,'pending',NULL,NULL,NULL,999,2000),
		(1001,'2026-09-08 19:30',60,'assigned',NULL,NULL,NULL,999,2000),
		(1001,'2026-09-08 20:00',60,'assigned',NULL,NULL,NULL,999,2000),
		(1002,'2026-09-08 12:00',60,'assigned',NULL,NULL,NULL,999,2000),
		(NULL,'2026-09-08 13:00',60,'pending',NULL,NULL,NULL,999,2000);
	`)
	require.NoError(t, err)
	svc := service.NewHotelDayViewService(access, repository.NewHotelDayViewRepository(tx), nil)
	h := handler.NewHotelDayViewHandler(svc)
	guard := handler.NewPartnerHotelHandler(access)
	for _, account := range []*model.PartnerHotelStaff{admin, staff} {
		token, err := auth.GenerateToken(int(*account.UserID), account.AccessRole, "day-view-test-key")
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/hotel/day-view?date=2026-09-08", nil).WithContext(ctx)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		middleware.AuthMiddleware(guard.RestrictHotelAccount(http.HandlerFunc(h.Get)), "day-view-test-key").ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
		var view model.HotelDayView
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &view))
		require.Len(t, view.Branches, 1)
		require.Len(t, view.Therapists, 1)
		require.Equal(t, "Therapist", view.Therapists[0].Name)
		require.Len(t, view.Therapists[0].BookedSlots, 3)
		require.True(t, view.Therapists[0].BookedSlots[0].Start.Equal(view.Start))
		require.True(t, view.Therapists[0].BookedSlots[2].End.Equal(view.End))
		for _, secret := range []string{"guest_name", "booking_id", "client_id", "hotel_name", "notes", "reference_code", "final_total", "Janice", "Secret", "PRIVATE", "private-therapist", "surname", "status"} {
			require.NotContains(t, rr.Body.String(), secret)
		}
	}
	// Detailed hotel booking records remain available only through the dedicated,
	// hotel-scoped bookings endpoint.
	_, err = tx.Exec(ctx, `CREATE TABLE services(service_id INTEGER PRIMARY KEY,name TEXT); ALTER TABLE bookings ADD COLUMN service_id INTEGER;`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO bookings(therapist_id,scheduled_start,duration_minutes,status,guest_name,notes,client_id) VALUES
 (1001,'2026-09-08 07:00',30,'pending','Hotel Guest','Client phone: 09171234567',$1),
 (1001,'2026-09-08 07:30',30,'assigned','Second Hotel Guest','Room number / hotel address: 204',$2)`, *admin.UserID, *staff.UserID)
	require.NoError(t, err)
	bookings := repository.NewHotelBookingRepository(tx)
	for _, account := range []*model.PartnerHotelStaff{admin, staff} {
		view, err := svc.Get(ctx, *account.UserID, "2026-09-08")
		require.NoError(t, err)
		encoded, err := json.Marshal(view)
		require.NoError(t, err)
		for _, secret := range []string{"Hotel Guest", "Second Hotel Guest", "09171234567", "Room number", "booking_id", "guest_name", "notes"} {
			require.NotContains(t, string(encoded), secret)
		}
		list, err := bookings.List(ctx, account.PartnerHotelID, 50, 0, "")
		require.NoError(t, err)
		require.Len(t, list, 2)
		require.Equal(t, "pending", list[0].Status)
		firstPage, err := bookings.List(ctx, account.PartnerHotelID, 1, 0, "")
		require.NoError(t, err)
		require.Len(t, firstPage, 1)
		require.Equal(t, list[0].BookingID, firstPage[0].BookingID)
		assigned, err := bookings.List(ctx, account.PartnerHotelID, 1, 0, "assigned")
		require.NoError(t, err)
		require.Len(t, assigned, 1)
		require.Equal(t, "assigned", assigned[0].Status)
		require.Equal(t, "Second Hotel Guest", assigned[0].GuestName)
		require.Contains(t, list[0].GuestName+list[1].GuestName, "Hotel Guest")
		owner, err := bookings.Owner(ctx, account.PartnerHotelID, list[0].BookingID)
		require.NoError(t, err)
		require.Contains(t, []int64{*admin.UserID, *staff.UserID}, owner)
		_, err = bookings.Owner(ctx, account.PartnerHotelID+9999, list[0].BookingID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
	}
}
