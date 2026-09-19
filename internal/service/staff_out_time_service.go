package service

import (
	"context"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type StaffOutTimeService interface {
	ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error)
	CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error)
	UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error)
	VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error
}

type staffOutTimeService struct {
	repo     repository.StaffOutTimeRepository
	location *time.Location
}

func NewStaffOutTimeService(repo repository.StaffOutTimeRepository) StaffOutTimeService {
	location, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		location = time.FixedZone("Asia/Manila", 8*60*60)
	}
	return &staffOutTimeService{repo: repo, location: location}
}

func (s *staffOutTimeService) ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error) {
	filter.Role = strings.TrimSpace(filter.Role)
	filter.Search = strings.TrimSpace(filter.Search)
	return s.repo.ListStaffOutTimes(ctx, filter)
}

func (s *staffOutTimeService) CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	prepared, err := s.prepareStaffOutTime(ctx, outTime)
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.CreateStaffOutTime(ctx, prepared)
	if err != nil {
		return nil, err
	}
	stored.FullName = prepared.FullName
	stored.Role = prepared.Role
	return stored, nil
}

func (s *staffOutTimeService) UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	prepared, err := s.prepareStaffOutTime(ctx, outTime)
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.UpdateStaffOutTime(ctx, prepared)
	if err != nil {
		return nil, err
	}
	stored.FullName = prepared.FullName
	stored.Role = prepared.Role
	return stored, nil
}

func (s *staffOutTimeService) VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error {
	return s.repo.VoidStaffOutTime(ctx, outTimeID, actorID)
}

func (s *staffOutTimeService) prepareStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (model.StaffOutTime, error) {
	target, err := s.repo.GetStaffOutTimeTargetUser(ctx, outTime.UserID)
	if err != nil {
		return model.StaffOutTime{}, err
	}
	if !isStaffOutTimeRoleEligible(target.Role) {
		return model.StaffOutTime{}, model.ErrInvalidStaffOutTimeTargetRole
	}
	if !s.outAtInsideWorkDateWindow(outTime.WorkDate, outTime.OutAt) {
		return model.StaffOutTime{}, model.ErrStaffOutTimeOutsideWorkDateWindow
	}

	outTime.FullName = target.FullName
	outTime.Role = target.Role
	outTime.Notes = strings.TrimSpace(outTime.Notes)
	return outTime, nil
}

func (s *staffOutTimeService) outAtInsideWorkDateWindow(workDate time.Time, outAt time.Time) bool {
	localOutAt := outAt.In(s.location)
	windowStart := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), 0, 0, 0, 0, s.location)
	windowEnd := windowStart.Add(36 * time.Hour)
	return !localOutAt.Before(windowStart) && !localOutAt.After(windowEnd)
}

func isStaffOutTimeRoleEligible(role string) bool {
	switch role {
	case model.RoleTherapist, model.RoleRider, model.RoleAdmin, model.RoleSuperAdmin:
		return true
	default:
		return false
	}
}
