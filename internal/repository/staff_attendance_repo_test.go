package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/mock"
)

func TestStaffAttendanceRepositoryIsAttendanceLockedChecksApprovedOrPaidRuns(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_attendance_details") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid") &&
			strings.Contains(sql, "voided_at is null")
	}), []interface{}{int64(77)}).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	locked, err := repo.IsAttendanceLocked(context.Background(), 77)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !locked {
		t.Fatalf("expected attendance to be locked")
	}
	mockDB.AssertExpectations(t)
}

func TestStaffAttendanceRepositoryGetActiveByUserDateMapsMissingRowToNotFound(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)
	workDate, err := time.Parse("2006-01-02", "2026-05-17")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_attendance_entries") &&
			strings.Contains(sql, "sae.user_id = $1") &&
			strings.Contains(sql, "sae.work_date = $2") &&
			strings.Contains(sql, "sae.voided_at is null")
	}), []interface{}{int64(88), "2026-05-17"}).Return(row).Once()
	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows).Once()

	attendance, err := repo.GetActiveStaffAttendanceByUserDate(context.Background(), 88, workDate)

	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
	if attendance != nil {
		t.Fatalf("expected nil attendance, got %#v", attendance)
	}
	mockDB.AssertExpectations(t)
}

func TestStaffAttendanceRepositoryCreateMapsActiveUserDateUniqueViolationToDuplicate(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)
	workDate, err := time.Parse("2006-01-02", "2026-05-17")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into staff_attendance_entries") &&
			strings.Contains(sql, "on conflict") &&
			strings.Contains(sql, "not exists")
	}), mock.Anything).Return(row).Once()
	row.On("Scan", staffAttendanceScanMockArgs()...).Return(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_staff_attendance_active_user_date",
	}).Once()

	attendance, err := repo.CreateStaffAttendance(context.Background(), model.StaffAttendance{
		UserID:   11,
		WorkDate: workDate,
	})

	if !errors.Is(err, model.ErrStaffAttendanceDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if attendance != nil {
		t.Fatalf("expected nil attendance, got %#v", attendance)
	}
	mockDB.AssertExpectations(t)
}

func TestStaffAttendanceRepositoryUpdateMapsActiveUserDateUniqueViolationToDuplicate(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)
	workDate, err := time.Parse("2006-01-02", "2026-05-17")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_attendance_entries") &&
			strings.Contains(sql, "not exists")
	}), mock.Anything).Return(row).Once()
	row.On("Scan", staffAttendanceScanMockArgs()...).Return(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_staff_attendance_active_user_date",
	}).Once()

	attendance, err := repo.UpdateStaffAttendance(context.Background(), model.StaffAttendance{
		AttendanceID: 44,
		UserID:       11,
		WorkDate:     workDate,
	})

	if !errors.Is(err, model.ErrStaffAttendanceDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if attendance != nil {
		t.Fatalf("expected nil attendance, got %#v", attendance)
	}
	mockDB.AssertExpectations(t)
}

func TestStaffAttendanceRepositoryUpdateMapsGuardedNoRowToLocked(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)
	workDate, err := time.Parse("2006-01-02", "2026-05-17")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_attendance_entries") &&
			strings.Contains(sql, "payroll_attendance_details") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid") &&
			strings.Contains(sql, "not exists")
	}), mock.Anything).Return(row).Once()
	row.On("Scan", staffAttendanceScanMockArgs()...).Return(pgx.ErrNoRows).Once()

	attendance, err := repo.UpdateStaffAttendance(context.Background(), model.StaffAttendance{
		AttendanceID: 44,
		UserID:       11,
		WorkDate:     workDate,
	})

	if !errors.Is(err, model.ErrStaffAttendanceLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	if attendance != nil {
		t.Fatalf("expected nil attendance, got %#v", attendance)
	}
	mockDB.AssertExpectations(t)
}

func TestStaffAttendanceRepositoryVoidMapsGuardedNoRowToLocked(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewStaffAttendanceRepository(mockDB)
	row := new(MockRow)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_attendance_entries") &&
			strings.Contains(sql, "payroll_attendance_details") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid") &&
			strings.Contains(sql, "not exists")
	}), []interface{}{int64(44), int64(9)}).Return(pgconn.NewCommandTag("UPDATE 0"), nil).Once()
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_attendance_details") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid")
	}), []interface{}{int64(44)}).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	err := repo.VoidStaffAttendance(context.Background(), 44, 9)

	if !errors.Is(err, model.ErrStaffAttendanceLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func staffAttendanceScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	}
}
