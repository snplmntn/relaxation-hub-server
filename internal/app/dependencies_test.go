package app

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestSendOpsAdminNotifications_UsesSingleBatchCreateForMultipleAdmins(t *testing.T) {
	admins := []model.User{{UserID: 11}, {UserID: 12}, {UserID: 13}}
	listCalls := 0
	createManyCalls := 0
	var captured []*model.CreateNotificationRequest

	err := sendOpsAdminNotifications(
		context.Background(),
		func(ctx context.Context) ([]model.User, error) {
			listCalls++
			return admins, nil
		},
		func(ctx context.Context, reqs []*model.CreateNotificationRequest) ([]*model.Notification, error) {
			createManyCalls++
			captured = reqs
			return nil, nil
		},
		"Database Pressure",
		map[string]string{"severity": "high"},
	)

	if err != nil {
		t.Fatalf("sendOpsAdminNotifications returned error: %v", err)
	}
	if listCalls != 1 || createManyCalls != 1 {
		t.Fatalf("expected one ListUsers and one CreateMany call, got list=%d createMany=%d", listCalls, createManyCalls)
	}
	if len(captured) != 3 {
		t.Fatalf("expected three notification requests, got %d", len(captured))
	}
	for i, req := range captured {
		if req.UserID != int64(admins[i].UserID) || req.Type != "ops_alert" || req.Title != "System Alert: Database Pressure" {
			t.Fatalf("unexpected request %d: %#v", i, req)
		}
		if req.Message != "Database Pressure; severity=high" {
			t.Fatalf("unexpected message for request %d: %q", i, req.Message)
		}
	}
}
