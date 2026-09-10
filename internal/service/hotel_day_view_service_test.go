package service

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type hotelDayViewTestRepo struct {
	calls      int
	start, end time.Time
}

func (r *hotelDayViewTestRepo) ListBranches(context.Context) ([]model.HotelDayViewBranch, error) {
	r.calls++
	return []model.HotelDayViewBranch{}, nil
}
func (r *hotelDayViewTestRepo) ListSchedule(_ context.Context, start, end time.Time, _ int64) ([]model.HotelDayViewTherapist, error) {
	r.calls++
	r.start = start
	r.end = end
	return []model.HotelDayViewTherapist{}, nil
}

func TestHotelDayViewRequiresActiveMembership(t *testing.T) {
	repo := &hotelDayViewTestRepo{}
	svc := NewHotelDayViewService(NewPartnerHotelService(&partnerHotelServiceRepo{}), repo)
	_, err := svc.Get(context.Background(), 7, "2026-09-08")
	require.ErrorIs(t, err, ErrHotelAccessDenied)
	require.Zero(t, repo.calls)
}

func TestHotelDayViewBusinessWindow(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff} {
		t.Run(role, func(t *testing.T) {
			repo := &hotelDayViewTestRepo{}
			svc := NewHotelDayViewService(NewPartnerHotelService(&partnerHotelServiceRepo{access: &model.HotelAccess{AccessRole: role}}), repo)
			view, err := svc.Get(context.Background(), 7, "2026-09-08")
			require.NoError(t, err)
			require.Equal(t, "2026-09-08T05:00:00Z", view.Start.Format(time.RFC3339))
			require.Equal(t, "2026-09-08T20:00:00Z", view.End.Format(time.RFC3339))
			require.Equal(t, view.Start, repo.start)
			require.Equal(t, view.End, repo.end)
			for _, date := range []string{"", "2026-02-30", "2026-09-08T00:00:00Z"} {
				_, err := svc.Get(context.Background(), 7, date)
				require.ErrorIs(t, err, ErrHotelDayViewDate)
			}
		})
	}
}

func TestHotelSlotsClipAndMergeWithoutExposingBookingBoundaries(t *testing.T) {
	start := time.Date(2026, 9, 8, 5, 0, 0, 0, time.UTC)
	end := start.Add(15 * time.Hour)
	merged := mergeHotelSlots([]model.HotelBookedSlot{
		{Start: start.Add(30 * time.Minute), End: start.Add(2 * time.Hour)},
		{Start: start.Add(-time.Hour), End: start.Add(time.Hour)},
		{Start: start.Add(2 * time.Hour), End: start.Add(3 * time.Hour)},
		{Start: end.Add(-time.Hour), End: end.Add(time.Hour)},
		{Start: end, End: end.Add(2 * time.Hour)},
	}, start, end)
	require.Equal(t, []model.HotelBookedSlot{{Start: start, End: start.Add(3 * time.Hour)}, {Start: end.Add(-time.Hour), End: end}}, merged)
}
