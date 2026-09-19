package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type StaffAttendanceService interface {
	ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error)
	ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error)
	CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error)
	UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error)
	VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64, actorRole string) error
}

type staffAttendanceService struct {
	repo     repository.StaffAttendanceRepository
	location *time.Location
}

func NewStaffAttendanceService(repo repository.StaffAttendanceRepository) StaffAttendanceService {
	location, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		location = time.FixedZone("Asia/Manila", 8*60*60)
	}
	return &staffAttendanceService{repo: repo, location: location}
}

func (s *staffAttendanceService) ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error) {
	filter.Role = strings.TrimSpace(filter.Role)
	filter.Search = strings.TrimSpace(filter.Search)
	return s.repo.ListStaffAttendance(ctx, filter)
}

func (s *staffAttendanceService) ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error) {
	search = strings.TrimSpace(search)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListStaffAttendanceAdminTargets(ctx, search, limit)
}

func (s *staffAttendanceService) CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error) {
	prepared, err := s.prepareStaffAttendance(ctx, attendance, actorID, actorRole)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetActiveStaffAttendanceByUserDate(ctx, prepared.UserID, prepared.WorkDate)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		locked, err := s.repo.IsAttendanceLocked(ctx, existing.AttendanceID)
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, model.ErrStaffAttendanceLocked
		}
	}
	stored, err := s.repo.CreateStaffAttendance(ctx, prepared)
	if err != nil {
		return nil, err
	}
	stored.FullName = prepared.FullName
	stored.Role = prepared.Role
	return stored, nil
}

func (s *staffAttendanceService) UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (*model.StaffAttendance, error) {
	existing, err := s.repo.GetStaffAttendance(ctx, attendance.AttendanceID)
	if err != nil {
		return nil, err
	}
	if actorRole == model.RoleAdmin && existing.UserID == actorID {
		return nil, model.ErrStaffAttendanceSelfEditForbidden
	}
	locked, err := s.repo.IsAttendanceLocked(ctx, attendance.AttendanceID)
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, model.ErrStaffAttendanceLocked
	}
	prepared, err := s.prepareStaffAttendance(ctx, attendance, actorID, actorRole)
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.UpdateStaffAttendance(ctx, prepared)
	if err != nil {
		return nil, err
	}
	stored.FullName = prepared.FullName
	stored.Role = prepared.Role
	return stored, nil
}

func (s *staffAttendanceService) VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64, actorRole string) error {
	locked, err := s.repo.IsAttendanceLocked(ctx, attendanceID)
	if err != nil {
		return err
	}
	if locked {
		return model.ErrStaffAttendanceLocked
	}
	existing, err := s.repo.GetStaffAttendance(ctx, attendanceID)
	if err != nil {
		return err
	}
	if actorRole == model.RoleAdmin && actorID == existing.UserID {
		return model.ErrStaffAttendanceSelfEditForbidden
	}
	return s.repo.VoidStaffAttendance(ctx, attendanceID, actorID)
}

func (s *staffAttendanceService) prepareStaffAttendance(ctx context.Context, attendance model.StaffAttendance, actorID int64, actorRole string) (model.StaffAttendance, error) {
	target, err := s.repo.GetStaffAttendanceTargetUser(ctx, attendance.UserID)
	if err != nil {
		return model.StaffAttendance{}, err
	}
	if !isStaffAttendanceRoleEligible(target.Role) {
		return model.StaffAttendance{}, model.ErrInvalidStaffAttendanceTargetRole
	}
	if actorRole == model.RoleAdmin && actorID == attendance.UserID {
		return model.StaffAttendance{}, model.ErrStaffAttendanceSelfEditForbidden
	}
	if err := s.validateStaffAttendanceTimes(attendance.WorkDate, attendance.TimeInAt, attendance.TimeOutAt); err != nil {
		return model.StaffAttendance{}, err
	}

	attendance.FullName = target.FullName
	attendance.Role = target.Role
	attendance.Notes = strings.TrimSpace(attendance.Notes)
	return attendance, nil
}

func (s *staffAttendanceService) validateStaffAttendanceTimes(workDate time.Time, timeInAt *time.Time, timeOutAt *time.Time) error {
	if timeInAt != nil && !s.attendanceTimeInsideWorkDateWindow(workDate, *timeInAt) {
		return model.ErrStaffAttendanceOutsideWorkDateWindow
	}
	if timeInAt != nil && timeOutAt != nil {
		if !timeOutAt.After(*timeInAt) {
			return model.ErrStaffAttendanceTimeOutBeforeTimeIn
		}
		if timeOutAt.Sub(*timeInAt) > 24*time.Hour {
			return model.ErrStaffAttendanceShiftTooLong
		}
	}
	return nil
}

func (s *staffAttendanceService) attendanceTimeInsideWorkDateWindow(workDate time.Time, value time.Time) bool {
	localValue := value.In(s.location)
	windowStart := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), 0, 0, 0, 0, s.location)
	windowEndExclusive := windowStart.Add(36 * time.Hour)
	return !localValue.Before(windowStart) && localValue.Before(windowEndExclusive)
}

func isStaffAttendanceRoleEligible(role string) bool {
	switch role {
	case model.RoleTherapist, model.RoleRider, model.RoleAdmin:
		return true
	default:
		return false
	}
}
