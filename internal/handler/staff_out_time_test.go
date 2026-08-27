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

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeStaffOutTimeService struct {
	items    []model.StaffOutTime
	create   model.StaffOutTime
	update   model.StaffOutTime
	voidID   int64
	voidBy   int64
	err      error
	voidErr  error
	listCall model.StaffOutTimeFilter
}

func (f *fakeStaffOutTimeService) ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error) {
	f.listCall = filter
	return f.items, f.err
}

func (f *fakeStaffOutTimeService) CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	f.create = outTime
	if f.err != nil {
		return nil, f.err
	}
	outTime.OutTimeID = 1
	return &outTime, nil
}

func (f *fakeStaffOutTimeService) UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	f.update = outTime
	if f.err != nil {
		return nil, f.err
	}
	return &outTime, nil
}

func (f *fakeStaffOutTimeService) VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error {
	f.voidID = outTimeID
	f.voidBy = actorID
	return f.voidErr
}

func TestStaffOutTimeHandlerListParsesFilters(t *testing.T) {
	service := &fakeStaffOutTimeService{}
	handler := NewStaffOutTimeHandler(service)

	req := httptest.NewRequest("GET", "/out-times?work_date=2026-05-17&role=rider&search=ana", nil)
	w := httptest.NewRecorder()

	handler.ListStaffOutTimes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.listCall.WorkDate.Format("2006-01-02") != "2026-05-17" || service.listCall.Role != model.RoleRider || service.listCall.Search != "ana" {
		t.Fatalf("unexpected filter: %#v", service.listCall)
	}
}

func TestStaffOutTimeHandlerCreateValidatesAndReturnsCreated(t *testing.T) {
	service := &fakeStaffOutTimeService{}
	handler := NewStaffOutTimeHandler(service)

	body := []byte(`{"user_id":11,"work_date":"2026-05-17","out_at":"2026-05-17T19:30:00+08:00","notes":"Done"}`)
	req := httptest.NewRequest("POST", "/out-times", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateStaffOutTime(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if service.create.UserID != 11 || service.create.WorkDate.Format("2006-01-02") != "2026-05-17" || service.create.Notes != "Done" {
		t.Fatalf("unexpected create payload: %#v", service.create)
	}
	var resp model.StaffOutTime
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.OutTimeID != 1 {
		t.Fatalf("expected created out_time_id, got %#v", resp)
	}
}

func TestStaffOutTimeHandlerMapsValidationAndNotFoundErrors(t *testing.T) {
	handler := NewStaffOutTimeHandler(&fakeStaffOutTimeService{err: model.ErrStaffOutTimeOutsideWorkDateWindow})
	body := []byte(`{"user_id":11,"work_date":"2026-05-17","out_at":"2026-05-19T19:30:00+08:00"}`)
	req := httptest.NewRequest("POST", "/out-times", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateStaffOutTime(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected validation 400, got %d body=%s", w.Code, w.Body.String())
	}

	handler = NewStaffOutTimeHandler(&fakeStaffOutTimeService{voidErr: model.ErrNotFound})
	req = httptest.NewRequest("DELETE", "/out-times/99", nil)
	req.SetPathValue("id", "99")
	w = httptest.NewRecorder()

	handler.DeleteStaffOutTime(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected delete 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStaffOutTimeHandlerRejectsInvalidPayloads(t *testing.T) {
	handler := NewStaffOutTimeHandler(&fakeStaffOutTimeService{})
	cases := [][]byte{
		[]byte(`{"user_id":0,"work_date":"2026-05-17","out_at":"2026-05-17T19:30:00+08:00"}`),
		[]byte(`{"user_id":11,"work_date":"bad","out_at":"2026-05-17T19:30:00+08:00"}`),
		[]byte(`{"user_id":11,"work_date":"2026-05-17","out_at":"bad"}`),
	}

	for _, body := range cases {
		req := httptest.NewRequest("POST", "/out-times", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler.CreateStaffOutTime(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", body, w.Code)
		}
	}
}

func TestStaffOutTimeHandlerMapsUnexpectedErrors(t *testing.T) {
	handler := NewStaffOutTimeHandler(&fakeStaffOutTimeService{err: errors.New("db down")})
	req := httptest.NewRequest("GET", "/out-times?work_date=2026-05-17", nil)
	w := httptest.NewRecorder()

	handler.ListStaffOutTimes(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

var _ = time.Time{}
