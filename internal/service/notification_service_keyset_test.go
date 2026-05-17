package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockNotificationRepoKeyset struct {
	mock.Mock
}

func (m *mockNotificationRepoKeyset) Create(ctx context.Context, n *model.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *mockNotificationRepoKeyset) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	args := m.Called(ctx, notifications)
	return args.Error(0)
}

func (m *mockNotificationRepoKeyset) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]model.Notification), args.Int(1), args.Error(2)
}

func (m *mockNotificationRepoKeyset) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	args := m.Called(ctx, userID, cursor, limit)
	return args.Get(0).([]model.Notification), args.Error(1)
}

func (m *mockNotificationRepoKeyset) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockNotificationRepoKeyset) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	args := m.Called(ctx, notificationID, userID)
	return args.Error(0)
}

func (m *mockNotificationRepoKeyset) MarkAllAsRead(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestNotificationServiceListByUserKeyset_TrimsLimitPlusOneAndSetsNextCursor(t *testing.T) {
	repo := new(mockNotificationRepoKeyset)
	svc := NewNotificationService(repo, nil, nil)
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	notifications := []model.Notification{
		{NotificationID: 5, UserID: 7, Type: "booking", Title: "5", CreatedAt: createdAt},
		{NotificationID: 4, UserID: 7, Type: "booking", Title: "4", CreatedAt: createdAt},
		{NotificationID: 3, UserID: 7, Type: "booking", Title: "3", CreatedAt: createdAt},
	}
	repo.On("ListByUserKeyset", mock.Anything, int64(7), (*model.KeysetCursor)(nil), 3).Return(notifications, nil).Once()

	page, err := svc.ListByUserKeyset(context.Background(), 7, nil, 2)

	assert.NoError(t, err)
	assert.Len(t, page.Notifications, 2)
	assert.True(t, page.HasMore)
	assert.Equal(t, 2, page.Limit)
	assert.Equal(t, createdAt, *page.NextCursorCreatedAt)
	assert.Equal(t, int64(4), *page.NextCursorID)
	repo.AssertNotCalled(t, "ListByUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
}

func TestNotificationServiceListByUserKeyset_SecondPageDoesNotRepeatCursorRow(t *testing.T) {
	repo := new(mockNotificationRepoKeyset)
	svc := NewNotificationService(repo, nil, nil)
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	cursor := &model.KeysetCursor{CreatedAt: createdAt, ID: 4}
	notifications := []model.Notification{
		{NotificationID: 3, UserID: 7, Type: "booking", Title: "3", CreatedAt: createdAt},
		{NotificationID: 2, UserID: 7, Type: "booking", Title: "2", CreatedAt: createdAt.Add(-time.Minute)},
	}
	repo.On("ListByUserKeyset", mock.Anything, int64(7), cursor, 3).Return(notifications, nil).Once()

	page, err := svc.ListByUserKeyset(context.Background(), 7, cursor, 2)

	assert.NoError(t, err)
	assert.False(t, page.HasMore)
	assert.Equal(t, []int64{3, 2}, []int64{page.Notifications[0].NotificationID, page.Notifications[1].NotificationID})
	repo.AssertExpectations(t)
}
