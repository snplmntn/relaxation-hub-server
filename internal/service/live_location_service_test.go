package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type stubLiveLocationRepo struct {
	locationByUserID map[int64]*model.LiveLocation
	getErr           error
	requestedUserIDs []int64
}

func (s *stubLiveLocationRepo) Upsert(context.Context, *model.LiveLocation) error {
	return nil
}

func (s *stubLiveLocationRepo) GetByUserID(_ context.Context, userID int64) (*model.LiveLocation, error) {
	s.requestedUserIDs = append(s.requestedUserIDs, userID)
	if s.getErr != nil {
		return nil, s.getErr
	}

	loc, ok := s.locationByUserID[userID]
	if !ok {
		return nil, pgx.ErrNoRows
	}

	return loc, nil
}

type stubBookingAccessRepo struct {
	booking             *model.Booking
	err                 error
	requestedBookingIDs []int64
}

func (s *stubBookingAccessRepo) GetByBookingID(_ context.Context, bookingID int64) (*model.Booking, error) {
	s.requestedBookingIDs = append(s.requestedBookingIDs, bookingID)
	if s.err != nil {
		return nil, s.err
	}
	return s.booking, nil
}

func TestLiveLocationServiceGetLocationForBooking_AllowsEligibleParticipants(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 19, 11, 0, 0, 0, time.UTC)
	therapistID := int64(22)
	clientID := int64(11)

	locationRepo := &stubLiveLocationRepo{
		locationByUserID: map[int64]*model.LiveLocation{
			therapistID: {
				LocationID:  5,
				UserID:      therapistID,
				Latitude:    14.55,
				Longitude:   121.02,
				LastUpdated: now,
			},
			clientID: {
				LocationID:  6,
				UserID:      clientID,
				Latitude:    14.56,
				Longitude:   121.03,
				LastUpdated: now,
			},
		},
	}

	testCases := []struct {
		name             string
		booking          *model.Booking
		requesterUserID  int64
		wantTargetUserID int64
	}{
		{
			name: "client gets therapist location during on_the_way",
			booking: &model.Booking{
				BookingID:   77,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusOnTheWay,
			},
			requesterUserID:  clientID,
			wantTargetUserID: therapistID,
		},
		{
			name: "therapist gets client location during arrived",
			booking: &model.Booking{
				BookingID:   78,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusArrived,
			},
			requesterUserID:  therapistID,
			wantTargetUserID: clientID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bookingRepo := &stubBookingAccessRepo{booking: tc.booking}
			svc := NewLiveLocationService(locationRepo, bookingRepo, nil)

			result, err := svc.GetLocationForBooking(context.Background(), tc.booking.BookingID, tc.requesterUserID)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if result.TargetUserID != tc.wantTargetUserID {
				t.Fatalf("expected target user %d, got %d", tc.wantTargetUserID, result.TargetUserID)
			}

			if result.Location == nil {
				t.Fatal("expected location to be returned")
			}

			if result.Location.UserID != tc.wantTargetUserID {
				t.Fatalf("expected location for user %d, got %d", tc.wantTargetUserID, result.Location.UserID)
			}
		})
	}
}

func TestLiveLocationServiceGetLocationForBooking_DeniesIneligibleAccess(t *testing.T) {
	t.Parallel()

	therapistID := int64(22)
	clientID := int64(11)

	testCases := []struct {
		name            string
		booking         *model.Booking
		requesterUserID int64
		wantErr         error
	}{
		{
			name: "non participant denied",
			booking: &model.Booking{
				BookingID:   79,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusOnTheWay,
			},
			requesterUserID: 99,
			wantErr:         ErrLiveLocationAccessDenied,
		},
		{
			name: "assigned status denied",
			booking: &model.Booking{
				BookingID:   80,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusAssigned,
			},
			requesterUserID: clientID,
			wantErr:         ErrLiveLocationAccessDenied,
		},
		{
			name: "in progress status denied",
			booking: &model.Booking{
				BookingID:   81,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusInProgress,
			},
			requesterUserID: therapistID,
			wantErr:         ErrLiveLocationAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			locationRepo := &stubLiveLocationRepo{}
			bookingRepo := &stubBookingAccessRepo{booking: tc.booking}
			svc := NewLiveLocationService(locationRepo, bookingRepo, nil)

			_, err := svc.GetLocationForBooking(context.Background(), tc.booking.BookingID, tc.requesterUserID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}

			if len(locationRepo.requestedUserIDs) != 0 {
				t.Fatalf("expected no live location lookup, got %v", locationRepo.requestedUserIDs)
			}
		})
	}
}

func TestLiveLocationServiceGetLocationForBooking_PropagatesNotFoundCases(t *testing.T) {
	t.Parallel()

	therapistID := int64(22)
	clientID := int64(11)

	t.Run("booking not found", func(t *testing.T) {
		t.Parallel()

		locationRepo := &stubLiveLocationRepo{}
		bookingRepo := &stubBookingAccessRepo{err: pgx.ErrNoRows}
		svc := NewLiveLocationService(locationRepo, bookingRepo, nil)

		_, err := svc.GetLocationForBooking(context.Background(), 82, clientID)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected pgx.ErrNoRows, got %v", err)
		}
	})

	t.Run("target location not found", func(t *testing.T) {
		t.Parallel()

		bookingRepo := &stubBookingAccessRepo{
			booking: &model.Booking{
				BookingID:   83,
				ClientID:    clientID,
				TherapistID: &therapistID,
				Status:      model.BookingStatusOnTheWay,
			},
		}
		locationRepo := &stubLiveLocationRepo{}
		svc := NewLiveLocationService(locationRepo, bookingRepo, nil)

		_, err := svc.GetLocationForBooking(context.Background(), 83, clientID)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected pgx.ErrNoRows, got %v", err)
		}

		if len(locationRepo.requestedUserIDs) != 1 || locationRepo.requestedUserIDs[0] != therapistID {
			t.Fatalf("expected target lookup for therapist %d, got %v", therapistID, locationRepo.requestedUserIDs)
		}
	})
}

func TestLiveLocationServiceGetLocationForBooking_LogsMetadataOnly(t *testing.T) {
	therapistID := int64(22)
	clientID := int64(11)

	locationRepo := &stubLiveLocationRepo{
		locationByUserID: map[int64]*model.LiveLocation{
			therapistID: {
				LocationID:  10,
				UserID:      therapistID,
				Latitude:    14.55,
				Longitude:   121.02,
				LastUpdated: time.Now(),
			},
		},
	}
	bookingRepo := &stubBookingAccessRepo{
		booking: &model.Booking{
			BookingID:   84,
			ClientID:    clientID,
			TherapistID: &therapistID,
			Status:      model.BookingStatusOnTheWay,
		},
	}

	var buf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(oldLogger)

	svc := NewLiveLocationService(locationRepo, bookingRepo, nil)
	if _, err := svc.GetLocationForBooking(context.Background(), 84, clientID); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logOutput := buf.String()
	for _, want := range []string{`"requester_user_id":11`, `"target_user_id":22`, `"booking_id":84`, `"decision":"allow"`} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("expected log output to contain %s, got %s", want, logOutput)
		}
	}

	for _, forbidden := range []string{"latitude", "longitude"} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("expected log output to omit %s, got %s", forbidden, logOutput)
		}
	}
}
