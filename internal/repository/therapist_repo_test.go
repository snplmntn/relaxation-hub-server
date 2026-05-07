package repository

import (
	"context"
	"strings"
	"testing"

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
