package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTherapistRepoList_AvailableOnlyRequiresActiveNonDeletedUsers(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewTherapistRepository(mockDB)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "tp.accept_assignments = true") &&
			strings.Contains(lower, "tp.is_verified = true") &&
			strings.Contains(lower, "u.account_status = 'active'") &&
			strings.Contains(lower, "u.deleted_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 0
	})).Return(rows, nil).Once()
	rows.On("Close").Return().Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()

	profiles, err := repo.List(context.Background(), true)

	assert.NoError(t, err)
	assert.Empty(t, profiles)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestTherapistRepoGetProfileScansHomeAddressID(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewTherapistRepository(mockDB)
	therapistID := int64(22)
	branchID := int64(33)
	homeAddressID := int64(44)
	now := time.Date(2026, time.May, 25, 9, 0, 0, 0, time.UTC)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from therapist_profiles") &&
			strings.Contains(lower, "home_address_id") &&
			strings.Contains(lower, "where tp.therapist_id = $1")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == therapistID
	})).Return(row).Once()
	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = therapistID
		// args.Get(1) is nickname (**string) — left nil
		*args.Get(2).(*string) = "active"
		*args.Get(3).(**int64) = &branchID
		*args.Get(4).(**int64) = &homeAddressID
		*args.Get(7).(*float64) = 4.8
		*args.Get(8).(*int) = 10
		*args.Get(9).(*int) = 20
		*args.Get(10).(*bool) = true
		*args.Get(11).(*bool) = true
		*args.Get(12).(*bool) = false
		*args.Get(13).(*time.Time) = now
		*args.Get(14).(*time.Time) = now
	}).Return(nil).Once()

	profile, err := repo.GetProfile(context.Background(), therapistID)

	assert.NoError(t, err)
	if assert.NotNil(t, profile) {
		assert.Equal(t, &homeAddressID, profile.HomeAddressID)
	}
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestTherapistRepoHasAvailableTherapistsRequiresQuantity(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewTherapistRepository(mockDB)
	windowStart := time.Date(2026, time.July, 6, 14, 30, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.July, 6, 16, 30, 0, 0, time.UTC)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "pending_holds") &&
			strings.Contains(lower, "greatest(available.count - pending_holds.count, 0) >= $3") &&
			strings.Contains(lower, "tp.accept_assignments = true") &&
			strings.Contains(lower, "b.therapist_id is null") &&
			strings.Contains(lower, "b.scheduled_start::timestamptz < $2::timestamptz") &&
			strings.Contains(lower, "> $1::timestamptz")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 &&
			args[0] == windowStart &&
			args[1] == windowEnd &&
			args[2] == 2
	})).Return(row).Once()
	row.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	available, err := repo.HasAvailableTherapists(context.Background(), windowStart, windowEnd, 2)

	assert.NoError(t, err)
	assert.True(t, available)
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestBookingRepoAssignTherapist_PrecheckRequiresActiveNonDeletedUser(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewBookingRepository(mockDB)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from therapist_profiles") &&
			strings.Contains(lower, "join users") &&
			strings.Contains(lower, "u.account_status = 'active'") &&
			strings.Contains(lower, "u.deleted_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == int64(56)
	})).Return(row).Once()
	row.On("Scan", mock.Anything).Return(pgx.ErrNoRows).Once()

	err := repo.AssignTherapist(context.Background(), 120, 56)

	assert.ErrorIs(t, err, ErrTherapistNotFound)
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}
