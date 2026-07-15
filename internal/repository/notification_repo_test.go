package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepoCreateMany_EmptyBatchNoOps(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewNotificationRepository(mockDB).(*notificationRepoImpl)

	err := repo.CreateMany(context.Background(), nil)

	require.NoError(t, err)
	mockDB.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
}

func TestNotificationRepoCreateMany_InsertsAllRowsWithSingleReturningStatement(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewNotificationRepository(mockDB).(*notificationRepoImpl)
	createdAt := time.Date(2026, time.May, 11, 12, 30, 0, 0, time.UTC)
	notifications := []*model.Notification{
		{UserID: 11, Type: "ops_alert", Title: "A", Message: "first", Data: []byte(`{"type":"ops_alert"}`)},
		{UserID: 12, Type: "ops_alert", Title: "B", Message: "second", Data: []byte(`{"type":"ops_alert"}`)},
	}

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into notifications") &&
			strings.Contains(lower, "values") &&
			strings.Contains(lower, "nullif($5, '')::jsonb") &&
			strings.Contains(lower, "nullif($10, '')::jsonb") &&
			strings.Contains(lower, "returning notification_id, is_read, created_at, updated_at")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 10 &&
			args[0] == int64(11) && args[1] == "ops_alert" && args[2] == "A" && args[3] == "first" && args[4] == `{"type":"ops_alert"}` &&
			args[5] == int64(12) && args[6] == "ops_alert" && args[7] == "B" && args[8] == "second" && args[9] == `{"type":"ops_alert"}`
	})).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*(args.Get(0).(*int64)) = 101
		*(args.Get(1).(*bool)) = false
		*(args.Get(2).(*time.Time)) = createdAt
		*(args.Get(3).(*time.Time)) = createdAt
	}).Return(nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*(args.Get(0).(*int64)) = 102
		*(args.Get(1).(*bool)) = false
		*(args.Get(2).(*time.Time)) = createdAt.Add(time.Second)
		*(args.Get(3).(*time.Time)) = createdAt.Add(time.Second)
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	err := repo.CreateMany(context.Background(), notifications)

	require.NoError(t, err)
	assert.Equal(t, int64(101), notifications[0].NotificationID)
	assert.Equal(t, int64(102), notifications[1].NotificationID)
	assert.Equal(t, createdAt, notifications[0].CreatedAt)
	assert.Equal(t, createdAt.Add(time.Second), notifications[1].CreatedAt)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestNotificationRepoCreateMany_QueryErrorReturnsClearError(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewNotificationRepository(mockDB).(*notificationRepoImpl)
	notifications := []*model.Notification{{UserID: 11, Type: "ops_alert", Title: "A", Message: "first"}}

	mockDB.On("Query", mock.Anything, mock.Anything, mock.Anything).Return((*MockRows)(nil), errors.New("insert failed")).Once()

	err := repo.CreateMany(context.Background(), notifications)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create notifications batch")
	mockDB.AssertExpectations(t)
}

func TestNotificationRepoListByUserKeyset_UsesDuplicateTimestampSafeCursorWithoutCount(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewNotificationRepository(mockDB).(*notificationRepoImpl)
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	cursor := &model.KeysetCursor{CreatedAt: createdAt, ID: 42}
	limit := 21

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from notifications") &&
			strings.Contains(lower, "user_id = $1") &&
			strings.Contains(lower, "created_at < $2") &&
			strings.Contains(lower, "created_at = $2 and notification_id < $3") &&
			strings.Contains(lower, "order by created_at desc, notification_id desc") &&
			strings.Contains(lower, "limit $4") &&
			!strings.Contains(lower, "count(*)") &&
			!strings.Contains(lower, "offset")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 4 && args[0] == int64(7) && args[1] == createdAt && args[2] == int64(42) && args[3] == limit
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	notifications, err := repo.ListByUserKeyset(context.Background(), 7, cursor, limit)

	assert.NoError(t, err)
	assert.Empty(t, notifications)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestNotificationRepoListByUserKeyset_FirstPageUsesBoundedOrderingWithoutCursor(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewNotificationRepository(mockDB).(*notificationRepoImpl)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from notifications") &&
			strings.Contains(lower, "where user_id = $1") &&
			strings.Contains(lower, "order by created_at desc, notification_id desc") &&
			strings.Contains(lower, "limit $2") &&
			!strings.Contains(lower, "count(*)") &&
			!strings.Contains(lower, "offset") &&
			!strings.Contains(lower, "created_at <")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == int64(7) && args[1] == 21
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	notifications, err := repo.ListByUserKeyset(context.Background(), 7, nil, 21)

	assert.NoError(t, err)
	assert.Empty(t, notifications)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
