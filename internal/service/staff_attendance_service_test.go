package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeStaffAttendanceRepo struct {
	targetUser model.StaffAttendanceUser
	admins     []model.StaffAttendanceUser
	targetErr  error
	existing   *model.StaffAttendance
	getErr     error
	locked     bool
	lockedErr  error
	created    *model.StaffAttendance
	updated    *model.StaffAttendance
	voidedID   int64
}

func (f *fakeStaffAttendanceRepo) ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error) {
	return nil, nil
}

func (f *fakeStaffAttendanceRepo) ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error) {
	return f.admins, nil
}

func (f *fakeStaffAttendanceRepo) GetStaffAttendanceTargetUser(ctx context.Context, userID int64) (*model.StaffAttendanceUser, error) {
	if f.targetErr != nil {
		return nil, f.targetErr
	}
	return &f.targetUser, nil
}

func (f *fakeStaffAttendanceRepo) GetStaffAttendance(ctx context.Context, attendanceID int64) (*model.StaffAttendance, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.existing == nil {
		return nil, model.ErrNotFound
	}
	copied := *f.existing
	return &copied, nil
}

func (f *fakeStaffAttendanceRepo) GetActiveStaffAttendanceByUserDate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffAttendance, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.existing == nil {
		return nil, model.ErrNotFound
	}
	copied := *f.existing
	return &copied, nil
}

func (f *fakeStaffAttendanceRepo) CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error) {
	copied := attendance
	f.created = &copied
	return &copied, nil
}

func (f *fakeStaffAttendanceRepo) UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error) {
	copied := attendance
	f.updated = &copied
	return &copied, nil
}

func (f *fakeStaffAttendanceRepo) VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64) error {
	f.voidedID = attendanceID
	return nil
}

func (f *fakeStaffAttendanceRepo) IsAttendanceLocked(ctx context.Context, attendanceID int64) (bool, error) {
	return f.locked, f.lockedErr
}

func TestStaffAttendanceServiceListsAdminTargets(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{admins: []model.StaffAttendanceUser{
		{UserID: 10, FullName: "Admin One", Role: model.RoleAdmin},
	}}
	service := NewStaffAttendanceService(repo)

	items, err := service.ListStaffAttendanceAdminTargets(context.Background(), " Admin ", 9999)
	if err != nil {
		t.Fatalf("list admin targets: %v", err)
	}
	if len(items) != 1 || items[0].UserID != 10 {
		t.Fatalf("unexpected admin targets: %#v", items)
	}
}

func TestStaffAttendanceServiceRejectsSelfEditByAdmin(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 10, FullName: "Admin User", Role: model.RoleAdmin}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:   10,
		WorkDate: workDate,
		TimeInAt: ptrTime(time.Date(2026, 5, 17, 9, 0, 0, 0, manilaTestLocation())),
	}, 10, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceSelfEditForbidden) {
		t.Fatalf("expected self-edit forbidden, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected no create, got %#v", repo.created)
	}
}

func TestStaffAttendanceServiceRejectsAdminSelfUpdate(t *testing.T) {
	workDate := mustAttendanceDate(t, "2026-05-17")
	repo := &fakeStaffAttendanceRepo{
		targetUser: model.StaffAttendanceUser{UserID: 10, FullName: "Admin User", Role: model.RoleAdmin},
		existing:   &model.StaffAttendance{AttendanceID: 22, UserID: 10, Role: model.RoleAdmin, WorkDate: workDate},
	}
	service := NewStaffAttendanceService(repo)

	_, err := service.UpdateStaffAttendance(context.Background(), model.StaffAttendance{
		AttendanceID: 22,
		UserID:       10,
		WorkDate:     workDate,
		TimeInAt:     ptrTime(time.Date(2026, 5, 17, 9, 0, 0, 0, manilaTestLocation())),
	}, 10, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceSelfEditForbidden) {
		t.Fatalf("expected self-edit forbidden, got %v", err)
	}
	if repo.updated != nil {
		t.Fatalf("expected no update, got %#v", repo.updated)
	}
}

func TestStaffAttendanceServiceRejectsAdminSelfUpdateWhenPayloadChangesUser(t *testing.T) {
	workDate := mustAttendanceDate(t, "2026-05-17")
	repo := &fakeStaffAttendanceRepo{
		targetUser: model.StaffAttendanceUser{UserID: 99, FullName: "Rider User", Role: model.RoleRider},
		existing:   &model.StaffAttendance{AttendanceID: 22, UserID: 10, Role: model.RoleAdmin, WorkDate: workDate},
	}
	service := NewStaffAttendanceService(repo)

	_, err := service.UpdateStaffAttendance(context.Background(), model.StaffAttendance{
		AttendanceID: 22,
		UserID:       99,
		WorkDate:     workDate,
		TimeInAt:     ptrTime(time.Date(2026, 5, 17, 9, 0, 0, 0, manilaTestLocation())),
	}, 10, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceSelfEditForbidden) {
		t.Fatalf("expected self-edit forbidden, got %v", err)
	}
	if repo.updated != nil {
		t.Fatalf("expected no update, got %#v", repo.updated)
	}
}

func TestStaffAttendanceServiceRejectsAdminSelfVoid(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{
		existing: &model.StaffAttendance{AttendanceID: 22, UserID: 10, Role: model.RoleAdmin},
	}
	service := NewStaffAttendanceService(repo)

	err := service.VoidStaffAttendance(context.Background(), 22, 10, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceSelfEditForbidden) {
		t.Fatalf("expected self-edit forbidden, got %v", err)
	}
	if repo.voidedID != 0 {
		t.Fatalf("expected no void, got id %d", repo.voidedID)
	}
}

func TestStaffAttendanceServiceRejectsIneligibleTargetRole(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 20, FullName: "Client User", Role: model.RoleClient}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:   20,
		WorkDate: workDate,
		TimeInAt: ptrTime(time.Date(2026, 5, 17, 9, 0, 0, 0, manilaTestLocation())),
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrInvalidStaffAttendanceTargetRole) {
		t.Fatalf("expected invalid role error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected no create, got %#v", repo.created)
	}
}

func TestStaffAttendanceServiceRejectsTimeInOutsideManilaWindow(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 21, FullName: "Rider User", Role: model.RoleRider}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:   21,
		WorkDate: workDate,
		TimeInAt: ptrTime(time.Date(2026, 5, 18, 12, 0, 0, 0, manilaTestLocation())),
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceOutsideWorkDateWindow) {
		t.Fatalf("expected outside-window error, got %v", err)
	}
}

func TestStaffAttendanceServiceRejectsTimeOutBeforeTimeIn(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 11, FullName: "Rider User", Role: model.RoleRider}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")
	timeIn := time.Date(2026, 5, 17, 10, 0, 0, 0, manilaTestLocation())
	timeOut := time.Date(2026, 5, 17, 9, 59, 0, 0, manilaTestLocation())

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:    11,
		WorkDate:  workDate,
		TimeInAt:  &timeIn,
		TimeOutAt: &timeOut,
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceTimeOutBeforeTimeIn) {
		t.Fatalf("expected time-out-before-time-in error, got %v", err)
	}
}

func TestStaffAttendanceServiceRejectsShiftOverTwentyFourHours(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 12, FullName: "Rider User", Role: model.RoleRider}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")
	timeIn := time.Date(2026, 5, 17, 8, 0, 0, 0, manilaTestLocation())
	timeOut := timeIn.Add(24*time.Hour + time.Minute)

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:    12,
		WorkDate:  workDate,
		TimeInAt:  &timeIn,
		TimeOutAt: &timeOut,
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceShiftTooLong) {
		t.Fatalf("expected shift-too-long error, got %v", err)
	}
}

func TestStaffAttendanceServiceAllowsCrossMidnightShift(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 13, FullName: "Therapist User", Role: model.RoleTherapist}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")
	timeIn := time.Date(2026, 5, 17, 22, 0, 0, 0, manilaTestLocation())
	timeOut := time.Date(2026, 5, 18, 6, 0, 0, 0, manilaTestLocation())

	created, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:    13,
		WorkDate:  workDate,
		TimeInAt:  &timeIn,
		TimeOutAt: &timeOut,
		Notes:     "  night shift  ",
	}, 1, model.RoleAdmin)
	if err != nil {
		t.Fatalf("expected cross-midnight shift to be accepted, got %v", err)
	}
	if created.Notes != "night shift" || created.Role != model.RoleTherapist || created.FullName != "Therapist User" {
		t.Fatalf("unexpected created attendance: %#v", created)
	}
}

func TestStaffAttendanceServiceAllowsTwentyFourHourCrossMidnightShift(t *testing.T) {
	repo := &fakeStaffAttendanceRepo{targetUser: model.StaffAttendanceUser{UserID: 15, FullName: "Rider User", Role: model.RoleRider}}
	service := NewStaffAttendanceService(repo)
	workDate := mustAttendanceDate(t, "2026-05-17")
	timeIn := time.Date(2026, 5, 17, 23, 0, 0, 0, manilaTestLocation())
	timeOut := time.Date(2026, 5, 18, 23, 0, 0, 0, manilaTestLocation())

	created, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:    15,
		WorkDate:  workDate,
		TimeInAt:  &timeIn,
		TimeOutAt: &timeOut,
	}, 1, model.RoleAdmin)
	if err != nil {
		t.Fatalf("expected 24h cross-midnight shift to be accepted, got %v", err)
	}
	if created == nil || created.TimeOutAt == nil || !created.TimeOutAt.Equal(timeOut) {
		t.Fatalf("unexpected created attendance: %#v", created)
	}
}

func TestStaffAttendanceServiceRejectsLockedAttendance(t *testing.T) {
	workDate := mustAttendanceDate(t, "2026-05-17")
	repo := &fakeStaffAttendanceRepo{
		targetUser: model.StaffAttendanceUser{UserID: 14, FullName: "Rider User", Role: model.RoleRider},
		existing:   &model.StaffAttendance{AttendanceID: 44, UserID: 14, Role: model.RoleRider, WorkDate: workDate},
		locked:     true,
	}
	service := NewStaffAttendanceService(repo)
	timeIn := time.Date(2026, 5, 17, 8, 0, 0, 0, manilaTestLocation())

	_, err := service.UpdateStaffAttendance(context.Background(), model.StaffAttendance{
		AttendanceID: 44,
		UserID:       14,
		WorkDate:     workDate,
		TimeInAt:     &timeIn,
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceLocked) {
		t.Fatalf("expected locked error on update, got %v", err)
	}

	err = service.VoidStaffAttendance(context.Background(), 44, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceLocked) {
		t.Fatalf("expected locked error on void, got %v", err)
	}
}

func TestStaffAttendanceServiceRejectsCreateWhenExistingAttendanceIsLocked(t *testing.T) {
	workDate := mustAttendanceDate(t, "2026-05-17")
	repo := &fakeStaffAttendanceRepo{
		targetUser: model.StaffAttendanceUser{UserID: 16, FullName: "Rider User", Role: model.RoleRider},
		existing:   &model.StaffAttendance{AttendanceID: 55, UserID: 16, WorkDate: workDate},
		locked:     true,
	}
	service := NewStaffAttendanceService(repo)
	timeIn := time.Date(2026, 5, 17, 8, 0, 0, 0, manilaTestLocation())

	_, err := service.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:   16,
		WorkDate: workDate,
		TimeInAt: &timeIn,
	}, 1, model.RoleAdmin)
	if !errors.Is(err, model.ErrStaffAttendanceLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected no create/upsert, got %#v", repo.created)
	}
}

func mustAttendanceDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse work date: %v", err)
	}
	return parsed
}

func manilaTestLocation() *time.Location {
	return time.FixedZone("Asia/Manila", 8*60*60)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
