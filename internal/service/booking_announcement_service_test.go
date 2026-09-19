package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/require"
)

type bookingAnnouncementRepoStub struct {
	created  *model.BookingAnnouncement
	resolved []model.ResolvedBookingAnnouncement
}

func (r *bookingAnnouncementRepoStub) List(context.Context) ([]model.BookingAnnouncement, error) {
	return nil, nil
}

func (r *bookingAnnouncementRepoStub) Create(_ context.Context, announcement *model.BookingAnnouncement) error {
	r.created = announcement
	return nil
}

func (r *bookingAnnouncementRepoStub) Update(context.Context, *model.BookingAnnouncement) error {
	return nil
}

func (r *bookingAnnouncementRepoStub) Delete(context.Context, int64) error { return nil }

func (r *bookingAnnouncementRepoStub) ChooseWinner(context.Context, int64, int64) (*model.BookingAnnouncement, error) {
	return nil, nil
}

func (r *bookingAnnouncementRepoStub) ResolveForBooking(context.Context, int64) ([]model.ResolvedBookingAnnouncement, error) {
	return r.resolved, nil
}

func TestBookingAnnouncementCreateNormalizesUnlimitedVariations(t *testing.T) {
	t.Parallel()

	repo := &bookingAnnouncementRepoStub{}
	svc := NewBookingAnnouncementService(repo)
	announcement, err := svc.Create(context.Background(), model.BookingAnnouncementRequest{
		Variations: []model.BookingAnnouncementVariationRequest{
			{Message: "  Available po ang bed cover for 50 pesos.  "},
			{Message: "Bed cover for PHP 50—buy now!"},
			{Message: " Add a fresh bed cover for only PHP 50. "},
		},
		StartDate: "2026-09-19",
		EndDate:   "2026-09-20",
	})

	require.NoError(t, err)
	require.Len(t, announcement.Variations, 3)
	require.Equal(t, "Available po ang bed cover for 50 pesos.", announcement.Variations[0].Message)
	require.Equal(t, "Add a fresh bed cover for only PHP 50.", announcement.Variations[2].Message)
	require.Equal(t, 3, announcement.Variations[2].Position)
	require.Same(t, announcement, repo.created)
}

func TestBookingAnnouncementCreateRejectsEmptyVariationsAndInvalidRange(t *testing.T) {
	t.Parallel()

	tests := []model.BookingAnnouncementRequest{
		{StartDate: "2026-09-19", EndDate: "2026-09-20"},
		{Variations: []model.BookingAnnouncementVariationRequest{{Message: "  "}}, StartDate: "2026-09-19", EndDate: "2026-09-20"},
		{Variations: []model.BookingAnnouncementVariationRequest{{Message: "Offer"}}, StartDate: "2026-09-20", EndDate: "2026-09-19"},
	}

	for _, req := range tests {
		repo := &bookingAnnouncementRepoStub{}
		_, err := NewBookingAnnouncementService(repo).Create(context.Background(), req)
		require.Error(t, err)
		require.Nil(t, repo.created)
	}
}

func TestChooseBookingAnnouncementVariationIsStableAndHonorsWinner(t *testing.T) {
	t.Parallel()

	announcement := model.BookingAnnouncement{
		AnnouncementID: 7,
		Variations: []model.BookingAnnouncementVariation{
			{VariationID: 11, Message: "Version one"},
			{VariationID: 12, Message: "Version two"},
			{VariationID: 13, Message: "Version three"},
		},
	}

	first := chooseBookingAnnouncementVariation(announcement, 42)
	require.Equal(t, int64(12), first.VariationID)
	require.Equal(t, first, chooseBookingAnnouncementVariation(announcement, 42))

	winnerID := int64(13)
	announcement.WinnerVariationID = &winnerID
	require.Equal(t, int64(13), chooseBookingAnnouncementVariation(announcement, 42).VariationID)
}

func TestBookingAnnouncementResolveReturnsStoredAssignments(t *testing.T) {
	t.Parallel()

	want := []model.ResolvedBookingAnnouncement{{AnnouncementID: 7, VariationID: 12, Message: "Version two"}}
	svc := NewBookingAnnouncementService(&bookingAnnouncementRepoStub{resolved: want})

	got, err := svc.ResolveForBooking(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
