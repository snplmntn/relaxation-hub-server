package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserRepoUpdateUser_InactiveStatusClearsTherapistAcceptAssignments(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	repo := NewUserRepository(mockDB)

	userID := int64(56)
	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update users") &&
			strings.Contains(lower, "account_status") &&
			strings.Contains(lower, "deleted_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == "inactive" && args[1] == userID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update therapist_profiles") &&
			strings.Contains(lower, "accept_assignments = false") &&
			strings.Contains(lower, "role = 'therapist'")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == userID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	err := repo.UpdateUser(context.Background(), userID, map[string]interface{}{"account_status": "inactive"})

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestUserRepoUpdateUser_RollsBackInactiveStatusWhenTherapistCascadeFails(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	repo := NewUserRepository(mockDB)

	userID := int64(56)
	cascadeErr := errors.New("cascade failed")
	sequence := 0
	mockDB.On("Begin", mock.Anything).Run(func(mock.Arguments) {
		sequence++
		assert.Equal(t, 1, sequence)
	}).Return(tx, nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update users") &&
			strings.Contains(lower, "account_status") &&
			strings.Contains(lower, "deleted_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == "inactive" && args[1] == userID
	})).Run(func(mock.Arguments) {
		sequence++
		assert.Equal(t, 2, sequence)
	}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update therapist_profiles") &&
			strings.Contains(lower, "accept_assignments = false") &&
			strings.Contains(lower, "role = 'therapist'")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == userID
	})).Run(func(mock.Arguments) {
		sequence++
		assert.Equal(t, 3, sequence)
	}).Return(pgconn.NewCommandTag("UPDATE 0"), cascadeErr).Once()
	tx.On("Rollback", mock.Anything).Run(func(mock.Arguments) {
		sequence++
		assert.Equal(t, 4, sequence)
	}).Return(nil).Once()

	err := repo.UpdateUser(context.Background(), userID, map[string]interface{}{"account_status": "inactive"})

	assert.ErrorIs(t, err, cascadeErr)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}
