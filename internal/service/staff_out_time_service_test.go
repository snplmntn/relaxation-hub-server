package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeStaffOutTimeRepo struct {
	targetUser model.StaffOutTimeUser
	targetErr  error
	created    *model.StaffOutTime
}

func (f *fakeStaffOutTimeRepo) ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error) {
	return nil, nil
}

func (f *fakeStaffOutTimeRepo) GetStaffOutTimeTargetUser(ctx context.Context, userID int64) (*model.StaffOutTimeUser, error) {
	if f.targetErr != nil {
		return nil, f.targetErr
	}
	return &f.targetUser, nil
}

func (f *fakeStaffOutTimeRepo) CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	copied := outTime
	f.created = &copied
	return &copied, nil
}

func (f *fakeStaffOutTimeRepo) UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	return &outTime, nil
}

func (f *fakeStaffOutTimeRepo) VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error {
	return nil
}

func TestStaffOutTimeServiceCreateRejectsIneligibleTargetRole(t *testing.T) {
	repo := &fakeStaffOutTimeRepo{
		targetUser: model.StaffOutTimeUser{
			UserID:   10,
			FullName: "Client User",
			Role:     model.RoleClient,
		},
	}
	service := NewStaffOutTimeService(repo)

	workDate, _ := time.Parse("2006-01-02", "2026-05-17")
	outAt := time.Date(2026, 5, 17, 19, 30, 0, 0, time.FixedZone("Asia/Manila", 8*60*60))
	_, err := service.CreateStaffOutTime(context.Background(), model.StaffOutTime{
		UserID:   10,
		WorkDate: workDate,
		OutAt:    outAt,
	})

	if !errors.Is(err, model.ErrInvalidStaffOutTimeTargetRole) {
		t.Fatalf("expected invalid target role error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected no record to be created, got %#v", repo.created)
	}
}

func TestStaffOutTimeServiceCreateValidatesManilaWorkDateWindow(t *testing.T) {
	repo := &fakeStaffOutTimeRepo{
		targetUser: model.StaffOutTimeUser{
			UserID:   11,
			FullName: "Rider User",
			Role:     model.RoleRider,
		},
	}
	service := NewStaffOutTimeService(repo)
	workDate, _ := time.Parse("2006-01-02", "2026-05-17")
	manila := time.FixedZone("Asia/Manila", 8*60*60)

	_, err := service.CreateStaffOutTime(context.Background(), model.StaffOutTime{
		UserID:   11,
		WorkDate: workDate,
		OutAt:    time.Date(2026, 5, 18, 12, 1, 0, 0, manila),
	})
	if !errors.Is(err, model.ErrStaffOutTimeOutsideWorkDateWindow) {
		t.Fatalf("expected out-of-window error, got %v", err)
	}

	created, err := service.CreateStaffOutTime(context.Background(), model.StaffOutTime{
		UserID:   11,
		WorkDate: workDate,
		OutAt:    time.Date(2026, 5, 18, 12, 0, 0, 0, manila),
		Notes:    "  late close  ",
	})
	if err != nil {
		t.Fatalf("expected noon next-day out time to be accepted, got %v", err)
	}
	if created.Notes != "late close" {
		t.Fatalf("expected notes to be trimmed, got %q", created.Notes)
	}
	if created.Role != model.RoleRider || created.FullName != "Rider User" {
		t.Fatalf("expected target user fields to be copied, got %#v", created)
	}
}
