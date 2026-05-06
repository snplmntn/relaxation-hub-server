package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

func TestAssignmentWorker_BroadcastsOffer(t *testing.T) {
	// Setup mocks
	mockQueue := &mockQueue{
		items: []repository.QueueItem{{BookingID: 100, Attempts: 0}},
	}
	mockBooking := &mockBookingRepoAW{
		bookings: map[int64]*mockBooking{
			100: {ClientID: 1, ServiceID: func() *int64 { i := int64(1); return &i }()},
		},
	}
	mockMatch := &mockMatch{
		result: []model.TherapistProfile{{TherapistID: 99}},
	}
	mockOffer := &mockOfferRepoForTest{} // defined in offers_test.go

	// Capture broadcasts
	var broadcastCalled bool
	var broadcastEvent string
	var broadcastUserID int64

	// Override socketio broadcast function
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()

	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error {
		broadcastCalled = true
		broadcastUserID = userID
		broadcastEvent = event
		return nil
	}

	worker := NewAssignmentWorker(&mockDB{}, mockQueue, mockBooking, nil, mockOffer, nil, nil, &mockTherapistRepoForTest{}, mockMatch, nil, nil)

	// Run one process cycle
	worker.processOnce(context.Background())

	// Assertions
	if !broadcastCalled {
		t.Fatal("Expected broadcaster.BroadcastToUser to be called")
	}
	if broadcastUserID != 99 {
		t.Errorf("Expected broadcast to therapist 99, got %d", broadcastUserID)
	}
	if broadcastEvent != "offered_to_therapist" {
		t.Errorf("Expected event 'offered_to_therapist', got '%s'", broadcastEvent)
	}
}

func TestBookingService_BroadcastsAcceptDecline(t *testing.T) {
	// Setup mocks
	mockOffer := &mockOfferRepoAccept{
		offers: map[int64]*model.BookingOffer{
			50: {OfferID: 1, BookingID: 200, TherapistID: 50, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)},
		},
	}
	mockRepo := &mockRepoAccept{}

	svc := NewBookingService(mockRepo, nil, nil, &nilAssignmentQueueRepo{}, &noTher{}, mockOffer, nil, nil, nil, nil, nil, nil, nil, nil)

	// Test Accept Broadcast
	var lastEvent string
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()

	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error {
		lastEvent = event
		return nil
	}

	if err := svc.AcceptBookingOffer(context.Background(), 50, 200); err != nil {
		t.Fatalf("AcceptBookingOffer failed: %v", err)
	}

	if lastEvent != "offer_accepted" {
		t.Errorf("Expected 'offer_accepted' event, got '%s'", lastEvent)
	}

	// Reset and Test Decline Broadcast
	// Re-add offer as pending for decline test
	mockOffer.offers[50].Status = "pending"
	lastEvent = ""

	if err := svc.DeclineBookingOffer(context.Background(), 50, 200); err != nil {
		t.Fatalf("DeclineBookingOffer failed: %v", err)
	}

	if lastEvent != "offer_declined" {
		t.Errorf("Expected 'offer_declined' event, got '%s'", lastEvent)
	}
}

func TestAssignmentWorker_BroadcastsExpiration(t *testing.T) {
	// Setup mocks
	mockQueue := &mockQueue{
		items: []repository.QueueItem{{BookingID: 101, Attempts: 0}},
	}
	mockBooking := &mockBookingRepoAW{
		bookings: map[int64]*mockBooking{
			101: {ClientID: 1, ServiceID: func() *int64 { i := int64(1); return &i }()},
		},
	}
	// Mock offer repo to return no active offers, but some expired ones
	mockOffer := &mockOfferRepo{
		expired: []model.BookingOffer{
			{OfferID: 77, BookingID: 101, TherapistID: 88, Status: "expired"},
		},
	}
	// Mock match to return nothing so we don't proceed to offer creation
	mockMatch := &mockMatch{
		result: []model.TherapistProfile{},
	}

	// Capture broadcasts
	var broadcastCalled bool
	var broadcastEvent string
	var broadcastUserID int64

	// Override socketio broadcast function
	originalBroadcast := broadcaster.BroadcastToUser
	defer func() { broadcaster.BroadcastToUser = originalBroadcast }()

	broadcaster.BroadcastToUser = func(userID int64, event string, data interface{}) error {
		broadcastCalled = true
		broadcastUserID = userID
		broadcastEvent = event
		return nil
	}

	worker := NewAssignmentWorker(&mockDB{}, mockQueue, mockBooking, nil, mockOffer, nil, nil, &mockTherapistRepoForTest{}, mockMatch, nil, nil)

	// Run one process cycle
	worker.processOnce(context.Background())

	// Assertions
	if !broadcastCalled {
		t.Fatal("Expected broadcaster.BroadcastToUser to be called for expiration")
	}
	if broadcastUserID != 88 {
		t.Errorf("Expected broadcast to therapist 88, got %d", broadcastUserID)
	}
	if broadcastEvent != "offer_expired" {
		t.Errorf("Expected event 'offer_expired', got '%s'", broadcastEvent)
	}
}
