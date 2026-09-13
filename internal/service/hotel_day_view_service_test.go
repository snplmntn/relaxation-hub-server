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
	branches   []model.HotelDayViewBranch
	therapists []model.HotelDayViewTherapist
}

func (r *hotelDayViewTestRepo) ListBranches(context.Context) ([]model.HotelDayViewBranch, error) {
	r.calls++
	return r.branches, nil
}
func (r *hotelDayViewTestRepo) ListSchedule(_ context.Context, start, end time.Time) ([]model.HotelDayViewTherapist, error) {
	r.calls++
	r.start = start
	r.end = end
	return r.therapists, nil
}

type hotelDayViewOrderRepoStub struct {
	order *model.DayViewTherapistOrder
}

func (r *hotelDayViewOrderRepoStub) GetByViewAndBusinessDate(context.Context, string, time.Time) (*model.DayViewTherapistOrder, error) {
	return r.order, nil
}

func TestHotelDayViewRequiresActiveMembership(t *testing.T) {
	repo := &hotelDayViewTestRepo{}
	svc := NewHotelDayViewService(NewPartnerHotelService(&partnerHotelServiceRepo{}), repo, nil)
	_, err := svc.Get(context.Background(), 7, "2026-09-08")
	require.ErrorIs(t, err, ErrHotelAccessDenied)
	require.Zero(t, repo.calls)
}

func TestHotelDayViewBusinessWindow(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff} {
		t.Run(role, func(t *testing.T) {
			repo := &hotelDayViewTestRepo{}
			svc := NewHotelDayViewService(NewPartnerHotelService(&partnerHotelServiceRepo{access: &model.HotelAccess{AccessRole: role}}), repo, nil)
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

func TestHotelDayViewReturnsFiveLeastBookedAnonymousTherapists(t *testing.T) {
	start := time.Date(2026, 9, 8, 5, 0, 0, 0, time.UTC)
	branchID := int64(1)
	therapist := func(id int64, minutes int) model.HotelDayViewTherapist {
		return model.HotelDayViewTherapist{
			TherapistID:       id,
			Name:              "Private name",
			BranchID:          &branchID,
			AcceptingBookings: true,
			BookedSlots: []model.HotelBookedSlot{{
				Start: start.Add(time.Duration(id) * time.Minute),
				End:   start.Add(time.Duration(id+int64(minutes)) * time.Minute),
			}},
		}
	}
	repo := &hotelDayViewTestRepo{
		branches: []model.HotelDayViewBranch{{BranchID: branchID, Name: "Main Branch"}},
		therapists: []model.HotelDayViewTherapist{
			therapist(10, 210), // Lourie
			therapist(20, 270), // Evelyn
			therapist(30, 150), // Mylene
			therapist(40, 90),  // Janice
			therapist(50, 120), // Phebie
			therapist(60, 210), // Jazz
			therapist(7, 120),  // Jobel
			therapist(80, 300), // Kriza
			therapist(9, 120),  // Malou
		},
	}
	inactive := therapist(1, 0)
	inactive.AcceptingBookings = false
	otherBranchID := int64(2)
	otherBranch := therapist(2, 0)
	otherBranch.BranchID = &otherBranchID
	repo.therapists = append(repo.therapists, inactive, otherBranch)
	order := &hotelDayViewOrderRepoStub{order: &model.DayViewTherapistOrder{
		TherapistIDs: []int64{10, 20, 30, 40, 50, 60, 7, 80, 9},
	}}
	svc := NewHotelDayViewService(
		NewPartnerHotelService(&partnerHotelServiceRepo{access: &model.HotelAccess{AccessRole: model.RoleHotelAdmin}}),
		repo,
		order,
	)

	view, err := svc.Get(context.Background(), 7, "2026-09-08")
	require.NoError(t, err)
	require.Equal(t, []int64{40, 50, 7, 9, 30}, []int64{
		view.Therapists[0].TherapistID,
		view.Therapists[1].TherapistID,
		view.Therapists[2].TherapistID,
		view.Therapists[3].TherapistID,
		view.Therapists[4].TherapistID,
	})
	for _, row := range view.Therapists {
		require.Equal(t, hotelTherapistLabel, row.Name)
		require.True(t, row.AcceptingBookings)
	}
}
