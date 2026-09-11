package service

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type hotelBookingTestRepo struct {
	hotelID int64
	calls   int
}

func (r *hotelBookingTestRepo) Analytics(_ context.Context, hotelID int64, _, _ time.Time) (*model.HotelAnalytics, error) {
	r.hotelID = hotelID
	r.calls++
	return &model.HotelAnalytics{Summary: model.HotelAnalyticsSummary{TotalBookings: 4, CompletedBookings: 3, CancelledBookings: 1, Revenue: 1500}}, nil
}

func (r *hotelBookingTestRepo) ListOptions(context.Context) ([]model.HotelBookingOption, error) {
	r.calls++
	return []model.HotelBookingOption{{PartnerHotelID: 42, HotelName: "Hotel"}}, nil
}
func (r *hotelBookingTestRepo) ListAnalyticsOptions(context.Context) ([]model.HotelAnalyticsOption, error) {
	r.calls++
	return []model.HotelAnalyticsOption{{PartnerHotelID: 42, HotelName: "Hotel", IsActive: true}}, nil
}

func (r *hotelBookingTestRepo) List(_ context.Context, id int64, limit, offset int) ([]model.HotelBooking, error) {
	r.hotelID = id
	r.calls++
	return []model.HotelBooking{}, nil
}
func (r *hotelBookingTestRepo) Owner(_ context.Context, id, bookingID int64) (int64, error) {
	r.hotelID = id
	r.calls++
	return 0, pgx.ErrNoRows
}
func (r *hotelBookingTestRepo) HotelNames(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func TestHotelBookingScope(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff} {
		t.Run(role, func(t *testing.T) {
			repo := &hotelBookingTestRepo{}
			svc := NewHotelBookingService(NewPartnerHotelService(&partnerHotelServiceRepo{access: &model.HotelAccess{PartnerHotelID: 42, AccessRole: role}}), repo, nil)
			_, err := svc.List(context.Background(), 7, 1)
			require.NoError(t, err)
			require.Equal(t, int64(42), repo.hotelID)
			err = svc.Update(context.Background(), 7, 999, model.UpdateHotelBookingRequest{GuestName: "Guest"})
			require.ErrorIs(t, err, pgx.ErrNoRows)
			err = svc.Cancel(context.Background(), 7, 999, "Guest requested")
			require.ErrorIs(t, err, pgx.ErrNoRows)
		})
	}
}
func TestOperationalHotelBookingOptions(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(nil, repo, nil)
	options, err := svc.ListOptions(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Hotel", options[0].HotelName)
	require.Equal(t, 1, repo.calls)
}
func TestOperationalHotelAnalyticsOptions(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(nil, repo, nil)
	options, err := svc.ListAnalyticsOptions(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Hotel", options[0].HotelName)
	require.True(t, options[0].IsActive)
	require.Equal(t, 1, repo.calls)
}
func TestHotelAnalyticsIsScopedAndCalculatesRates(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(NewPartnerHotelService(&partnerHotelServiceRepo{access: &model.HotelAccess{PartnerHotelID: 42, AccessRole: model.RoleHotelAdmin}}), repo, nil)
	analytics, err := svc.Analytics(context.Background(), 7, 30)
	require.NoError(t, err)
	require.Equal(t, int64(42), repo.hotelID)
	require.Equal(t, 500.0, analytics.Summary.AverageBookingValue)
	require.Equal(t, 75.0, analytics.Summary.CompletionRate)
	require.Equal(t, 25.0, analytics.Summary.CancellationRate)
	require.Len(t, analytics.Period.From, 10)
	require.Len(t, analytics.Period.To, 10)
}

func TestOperationalHotelAnalyticsUsesSelectedHotel(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(nil, repo, nil)

	analytics, err := svc.AnalyticsForHotel(context.Background(), 91, 90)

	require.NoError(t, err)
	require.Equal(t, int64(91), repo.hotelID)
	require.Equal(t, 500.0, analytics.Summary.AverageBookingValue)
	require.Equal(t, 75.0, analytics.Summary.CompletionRate)
	require.Equal(t, 25.0, analytics.Summary.CancellationRate)
	require.Len(t, analytics.Period.From, 10)
	require.Len(t, analytics.Period.To, 10)
}

func TestHotelAnalyticsRejectsUnsupportedRange(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(NewPartnerHotelService(&partnerHotelServiceRepo{}), repo, nil)
	_, err := svc.Analytics(context.Background(), 7, 14)
	require.EqualError(t, err, "analytics range must be 7, 30, 90, or 365 days")
	require.Zero(t, repo.calls)
}

func TestOperationalHotelAnalyticsRejectsInvalidHotel(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(nil, repo, nil)

	_, err := svc.AnalyticsForHotel(context.Background(), 0, 30)

	require.EqualError(t, err, "hotel ID must be positive")
	require.Zero(t, repo.calls)
}
func TestHotelBookingsRejectInactiveMembership(t *testing.T) {
	repo := &hotelBookingTestRepo{}
	svc := NewHotelBookingService(NewPartnerHotelService(&partnerHotelServiceRepo{}), repo, nil)
	_, err := svc.List(context.Background(), 7, 1)
	require.ErrorIs(t, err, ErrHotelAccessDenied)
	require.ErrorIs(t, svc.Update(context.Background(), 7, 8, model.UpdateHotelBookingRequest{}), ErrHotelAccessDenied)
	require.ErrorIs(t, svc.Cancel(context.Background(), 7, 8, "Cancel"), ErrHotelAccessDenied)
	require.Zero(t, repo.calls)
}
