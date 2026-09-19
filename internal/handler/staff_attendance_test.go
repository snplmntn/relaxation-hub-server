package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeStaffAttendanceService struct {
	items    []model.StaffAttendance
	admins   []model.StaffAttendanceUser
	create   model.StaffAttendance
	update   model.StaffAttendance
	voidID   int64
	voidBy   int64
	err      error
	voidErr  error
	listCall model.StaffAttendanceFilter
	actorID  int64
	role     string
}

func (f *fakeStaffAttendanceService) ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error) {
	f.listCall = filter
	return f.items, f.err
}

func (f *fakeStaffAttendanceService) ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error) {
	return f.admins, f.err
}

func (f *fakeStaffAttendanceService) CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error) {
	f.create = attendance
	f.actorID = actorID
	f.role = actorRole
	if f.err != nil {
		return nil, f.err
	}
	attendance.AttendanceID = 1
	return &attendance, nil
}

func (f *fakeStaffAttendanceService) UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error) {
	f.update = attendance
	f.actorID = actorID
	f.role = actorRole
	if f.err != nil {
		return nil, f.err
	}
	return &attendance, nil
}

func (f *fakeStaffAttendanceService) VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64, actorRole string) error {
	f.voidID = attendanceID
	f.voidBy = actorID
	f.role = actorRole
	return f.voidErr
}

func TestStaffAttendanceHandlerListParsesFilters(t *testing.T) {
	service := &fakeStaffAttendanceService{}
	handler := NewStaffAttendanceHandler(service)

	req := httptest.NewRequest("GET", "/attendance?work_date=2026-05-17&role=rider&search=ana", nil)
	w := httptest.NewRecorder()

	handler.ListStaffAttendance(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.listCall.WorkDate.Format("2006-01-02") != "2026-05-17" || service.listCall.Role != model.RoleRider || service.listCall.Search != "ana" {
		t.Fatalf("unexpected filter: %#v", service.listCall)
	}
}

func TestStaffAttendanceHandlerListAdminTargets(t *testing.T) {
	service := &fakeStaffAttendanceService{admins: []model.StaffAttendanceUser{
		{UserID: 10, FullName: "Admin One", Role: model.RoleAdmin},
	}}
	handler := NewStaffAttendanceHandler(service)

	req := httptest.NewRequest("GET", "/attendance/staff?search=admin&limit=50", nil)
	w := httptest.NewRecorder()

	handler.ListStaffAttendanceAdminTargets(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []model.StaffAttendanceUser `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].UserID != 10 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestStaffAttendanceHandlerCreateValidatesAndReturnsCreated(t *testing.T) {
	service := &fakeStaffAttendanceService{}
	handler := NewStaffAttendanceHandler(service)

	body := []byte(`{"user_id":11,"work_date":"2026-05-17","time_in_at":"2026-05-17T09:00:00+08:00","time_out_at":"2026-05-17T18:00:00+08:00","notes":" Done "}`)
	req := staffAttendanceRequest("POST", "/attendance", body)
	w := httptest.NewRecorder()

	handler.CreateStaffAttendance(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if service.create.UserID != 11 || service.create.WorkDate.Format("2006-01-02") != "2026-05-17" || service.create.Notes != "Done" {
		t.Fatalf("unexpected create payload: %#v", service.create)
	}
	if service.create.TimeInAt == nil || service.create.TimeOutAt == nil {
		t.Fatalf("expected timestamps to be parsed: %#v", service.create)
	}
	var resp model.StaffAttendance
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AttendanceID != 1 {
		t.Fatalf("expected created attendance_id, got %#v", resp)
	}
}

func TestStaffAttendanceHandlerRejectsInvalidPayloads(t *testing.T) {
	handler := NewStaffAttendanceHandler(&fakeStaffAttendanceService{})
	cases := [][]byte{
		[]byte(`{"user_id":0,"work_date":"2026-05-17","time_in_at":"2026-05-17T09:00:00+08:00"}`),
		[]byte(`{"user_id":11,"work_date":"bad","time_in_at":"2026-05-17T09:00:00+08:00"}`),
		[]byte(`{"user_id":11,"work_date":"2026-05-17","time_in_at":"bad"}`),
		[]byte(`{"user_id":11,"work_date":"2026-05-17","time_in_at":"2026-05-17T09:00:00+08:00","extra":true}`),
	}

	for _, body := range cases {
		req := staffAttendanceRequest("POST", "/attendance", body)
		w := httptest.NewRecorder()
		handler.CreateStaffAttendance(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d body=%s", body, w.Code, w.Body.String())
		}
	}
}

func TestStaffAttendanceHandlerMapsValidationSelfEditLockedAndNotFoundErrors(t *testing.T) {
	body := []byte(`{"user_id":11,"work_date":"2026-05-17","time_out_at":"2026-05-17T18:00:00+08:00"}`)
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "validation", err: model.ErrStaffAttendanceOutsideWorkDateWindow, want: http.StatusBadRequest},
		{name: "self edit", err: model.ErrStaffAttendanceSelfEditForbidden, want: http.StatusForbidden},
		{name: "locked", err: model.ErrStaffAttendanceLocked, want: http.StatusConflict},
		{name: "duplicate", err: model.ErrStaffAttendanceDuplicate, want: http.StatusConflict},
		{name: "not found", err: model.ErrNotFound, want: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewStaffAttendanceHandler(&fakeStaffAttendanceService{err: tc.err})
			req := staffAttendanceRequest("POST", "/attendance", body)
			w := httptest.NewRecorder()

			handler.CreateStaffAttendance(w, req)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d body=%s", tc.want, w.Code, w.Body.String())
			}
		})
	}

	handler := NewStaffAttendanceHandler(&fakeStaffAttendanceService{voidErr: model.ErrNotFound})
	req := staffAttendanceRequest("DELETE", "/attendance/99", nil)
	req.SetPathValue("id", "99")
	w := httptest.NewRecorder()

	handler.DeleteStaffAttendance(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected delete 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStaffAttendanceHandlerDeletePassesActorRole(t *testing.T) {
	service := &fakeStaffAttendanceService{}
	handler := NewStaffAttendanceHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(5, model.RoleAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := staffAttendanceRequest("DELETE", "/attendance/99", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("id", "99")
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.DeleteStaffAttendance), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if service.voidID != 99 || service.voidBy != 5 || service.role != model.RoleAdmin {
		t.Fatalf("unexpected delete call: id=%d actor=%d role=%q", service.voidID, service.voidBy, service.role)
	}
}

func TestStaffAttendanceHandlerMapsUnexpectedErrors(t *testing.T) {
	handler := NewStaffAttendanceHandler(&fakeStaffAttendanceService{err: errors.New("db down")})
	req := httptest.NewRequest("GET", "/attendance?work_date=2026-05-17", nil)
	w := httptest.NewRecorder()

	handler.ListStaffAttendance(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func staffAttendanceRequest(method string, target string, body []byte) *http.Request {
	return httptest.NewRequest(method, target, bytes.NewReader(body))
}

var _ = time.Time{}
