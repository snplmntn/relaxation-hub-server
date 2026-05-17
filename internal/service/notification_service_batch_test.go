package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type batchNotificationRepo struct {
	createCalls     int
	createManyCalls int
	created         []*model.Notification
	createManyErr   error
}

func (r *batchNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	r.createCalls++
	n.NotificationID = int64(100 + r.createCalls)
	n.CreatedAt = time.Date(2026, time.May, 11, 12, 0, 0, 0, time.UTC)
	n.UpdatedAt = n.CreatedAt
	r.created = append(r.created, n)
	return nil
}

func (r *batchNotificationRepo) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	r.createManyCalls++
	if r.createManyErr != nil {
		return r.createManyErr
	}
	for i, notification := range notifications {
		notification.NotificationID = int64(200 + i)
		notification.CreatedAt = time.Date(2026, time.May, 11, 12, 0, i, 0, time.UTC)
		notification.UpdatedAt = notification.CreatedAt
		r.created = append(r.created, notification)
	}
	return nil
}

func (r *batchNotificationRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	return nil, 0, nil
}

func (r *batchNotificationRepo) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	return nil, nil
}

func (r *batchNotificationRepo) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (r *batchNotificationRepo) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return nil
}

func (r *batchNotificationRepo) MarkAllAsRead(ctx context.Context, userID int64) error {
	return nil
}

func TestNotificationServiceCreateMany_EmptyBatchNoOps(t *testing.T) {
	repo := &batchNotificationRepo{}
	svc := NewNotificationService(repo, nil, nil)

	notifications, err := svc.CreateMany(context.Background(), nil)

	if err != nil {
		t.Fatalf("CreateMany returned error: %v", err)
	}
	if len(notifications) != 0 {
		t.Fatalf("expected empty result, got %d", len(notifications))
	}
	if repo.createCalls != 0 || repo.createManyCalls != 0 {
		t.Fatalf("empty batch must not call repository, got create=%d createMany=%d", repo.createCalls, repo.createManyCalls)
	}
}

func TestNotificationServiceCreate_PreservesSingleRepositoryPath(t *testing.T) {
	repo := &batchNotificationRepo{}
	svc := NewNotificationService(repo, nil, nil)

	notification, err := svc.Create(context.Background(), &model.CreateNotificationRequest{
		UserID:  7,
		Type:    "ops_alert",
		Title:   " System Alert ",
		Message: " Check database ",
		Data:    map[string]any{"source": "single"},
	})

	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.createCalls != 1 || repo.createManyCalls != 0 {
		t.Fatalf("expected one Create repository call, got create=%d createMany=%d", repo.createCalls, repo.createManyCalls)
	}
	if notification.NotificationID != 101 || notification.Type != "ops_alert" || notification.Title != "System Alert" || notification.Message != "Check database" {
		t.Fatalf("single notification fields were not preserved: %#v", notification)
	}
	var data map[string]any
	if err := json.Unmarshal(notification.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal notification data: %v", err)
	}
	if data["notification_type"] != "ops_alert" || data["type"] != "ops_alert" || data["source"] != "single" {
		t.Fatalf("single notification data was not enriched: %#v", data)
	}
}

func TestNotificationServiceCreateMany_OnePreservesSingleCreateBehavior(t *testing.T) {
	repo := &batchNotificationRepo{}
	svc := NewNotificationService(repo, nil, nil)

	notifications, err := svc.CreateMany(context.Background(), []*model.CreateNotificationRequest{{
		UserID:  7,
		Type:    " ops_alert ",
		Title:   " System Alert ",
		Message: " Check database ",
		Data:    map[string]any{"source": "test"},
	}})

	if err != nil {
		t.Fatalf("CreateMany returned error: %v", err)
	}
	if repo.createManyCalls != 1 || repo.createCalls != 0 {
		t.Fatalf("expected one CreateMany repository call, got create=%d createMany=%d", repo.createCalls, repo.createManyCalls)
	}
	if len(notifications) != 1 || notifications[0].NotificationID != 200 {
		t.Fatalf("expected one enriched notification with repository output, got %#v", notifications)
	}
	var data map[string]any
	if err := json.Unmarshal(notifications[0].Data, &data); err != nil {
		t.Fatalf("failed to unmarshal notification data: %v", err)
	}
	if notifications[0].UserID != 7 || notifications[0].Type != "ops_alert" || notifications[0].Title != "System Alert" || notifications[0].Message != "Check database" {
		t.Fatalf("notification fields were not normalized: %#v", notifications[0])
	}
	if data["notification_type"] != "ops_alert" || data["type"] != "ops_alert" || data["notification_schema"] == "" || data["source"] != "test" {
		t.Fatalf("notification data was not enriched: %#v", data)
	}
}

func TestNotificationServiceCreateMany_ManyUsesOneRepositoryCall(t *testing.T) {
	repo := &batchNotificationRepo{}
	svc := NewNotificationService(repo, nil, nil)

	notifications, err := svc.CreateMany(context.Background(), []*model.CreateNotificationRequest{
		{UserID: 7, Type: "ops_alert", Title: "A", Message: "first"},
		{UserID: 8, NotificationType: "ops_alert", Title: "B", Message: "second"},
	})

	if err != nil {
		t.Fatalf("CreateMany returned error: %v", err)
	}
	if repo.createManyCalls != 1 || repo.createCalls != 0 {
		t.Fatalf("expected one CreateMany repository call, got create=%d createMany=%d", repo.createCalls, repo.createManyCalls)
	}
	if len(notifications) != 2 || notifications[0].NotificationID != 200 || notifications[1].NotificationID != 201 {
		t.Fatalf("expected two repository-enriched notifications, got %#v", notifications)
	}
}

func TestNotificationServiceCreateMany_RepositoryErrorReturnsError(t *testing.T) {
	repo := &batchNotificationRepo{createManyErr: errors.New("database unavailable")}
	svc := NewNotificationService(repo, nil, nil)

	notifications, err := svc.CreateMany(context.Background(), []*model.CreateNotificationRequest{{UserID: 7, Type: "ops_alert", Title: "A", Message: "first"}})

	if err == nil {
		t.Fatal("expected CreateMany to return repository error")
	}
	if notifications != nil {
		t.Fatalf("expected nil notifications on error, got %#v", notifications)
	}
	if repo.createManyCalls != 1 || repo.createCalls != 0 {
		t.Fatalf("expected one CreateMany repository call, got create=%d createMany=%d", repo.createCalls, repo.createManyCalls)
	}
}
