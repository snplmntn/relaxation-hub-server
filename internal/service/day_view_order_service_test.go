package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type dayViewOrderRepoStub struct {
	candidates []model.DayViewTherapistCandidate
	hours      map[int64]float64
	existing   *model.DayViewTherapistOrder
	upserts    []*model.DayViewTherapistOrder
}

func (s *dayViewOrderRepoStub) GetByViewAndBusinessDate(context.Context, string, time.Time) (*model.DayViewTherapistOrder, error) {
	if s.existing != nil {
		return s.existing, nil
	}
	return nil, pgx.ErrNoRows
}

func (s *dayViewOrderRepoStub) Upsert(_ context.Context, order *model.DayViewTherapistOrder) error {
	s.upserts = append(s.upserts, order)
	return nil
}

func (s *dayViewOrderRepoStub) ListTherapistsByBranch(context.Context, *int64) ([]model.DayViewTherapistCandidate, error) {
	return append([]model.DayViewTherapistCandidate(nil), s.candidates...), nil
}

func (s *dayViewOrderRepoStub) GetTherapistHoursBetween(context.Context, []int64, time.Time, time.Time) (map[int64]float64, error) {
	return s.hours, nil
}

func TestGenerateAutoOrderSortsYesterdayHoursAscending(t *testing.T) {
	repo := &dayViewOrderRepoStub{
		candidates: []model.DayViewTherapistCandidate{
			{TherapistID: 20, Name: "Cara"},
			{TherapistID: 30, Name: "Bea"},
			{TherapistID: 10, Name: "Ana"},
			{TherapistID: 40, Name: "Bea"},
		},
		hours: map[int64]float64{
			10: 1.5,
			20: 6,
			30: 1.5,
			// Therapist 40 has no visible booking hours and therefore defaults to zero.
		},
	}
	service := NewDayViewOrderService(repo)
	businessDate := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.FixedZone("Asia/Manila", 8*60*60))

	order, err := service.generateAutoOrder(context.Background(), dayViewScope{ViewKey: "freelance"}, businessDate)
	if err != nil {
		t.Fatalf("generateAutoOrder returned error: %v", err)
	}

	want := []int64{40, 10, 30, 20}
	if !reflect.DeepEqual(order.TherapistIDs, want) {
		t.Fatalf("therapist order = %v, want %v", order.TherapistIDs, want)
	}
	if order.Source != DayViewOrderSourceAuto {
		t.Fatalf("source = %q, want %q", order.Source, DayViewOrderSourceAuto)
	}
}

func TestGetOrGenerateOrderRefreshesStaleAutomaticOrder(t *testing.T) {
	repo := &dayViewOrderRepoStub{
		existing: &model.DayViewTherapistOrder{
			ViewKey:      "branch:1",
			TherapistIDs: []int64{1, 2, 3},
			Source:       DayViewOrderSourceAuto,
		},
		candidates: []model.DayViewTherapistCandidate{
			{TherapistID: 1, Name: "Phebie"},
			{TherapistID: 2, Name: "Jewel"},
			{TherapistID: 3, Name: "Jinky"},
		},
		hours: map[int64]float64{1: 4.5},
	}
	service := NewDayViewOrderService(repo)

	order, err := service.GetOrGenerateOrder(context.Background(), "branch:1")
	if err != nil {
		t.Fatalf("GetOrGenerateOrder returned error: %v", err)
	}

	want := []int64{2, 3, 1}
	if !reflect.DeepEqual(order.TherapistIDs, want) {
		t.Fatalf("therapist order = %v, want %v", order.TherapistIDs, want)
	}
	if len(repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(repo.upserts))
	}
}

func TestGetOrGenerateOrderPreservesManualOrder(t *testing.T) {
	existing := &model.DayViewTherapistOrder{
		ViewKey:      "branch:1",
		TherapistIDs: []int64{3, 1, 2},
		Source:       DayViewOrderSourceManual,
	}
	repo := &dayViewOrderRepoStub{existing: existing}
	service := NewDayViewOrderService(repo)

	order, err := service.GetOrGenerateOrder(context.Background(), "branch:1")
	if err != nil {
		t.Fatalf("GetOrGenerateOrder returned error: %v", err)
	}
	if order != existing {
		t.Fatal("manual order was not preserved")
	}
	if len(repo.upserts) != 0 {
		t.Fatalf("upserts = %d, want 0", len(repo.upserts))
	}
}

func TestBusinessDateAtUsesDayViewFourAMCutoff(t *testing.T) {
	location, err := time.LoadLocation(manilaLocationName)
	if err != nil {
		t.Fatalf("load Manila location: %v", err)
	}

	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{
			name: "before cutoff belongs to prior day",
			now:  time.Date(2026, time.July, 21, 3, 59, 0, 0, location),
			want: "2026-07-20",
		},
		{
			name: "cutoff starts new business day",
			now:  time.Date(2026, time.July, 21, 4, 0, 0, 0, location),
			want: "2026-07-21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := businessDateAt(tt.now, location).Format("2006-01-02"); got != tt.want {
				t.Fatalf("business date = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestYesterdayWindowUTCMatchesPreviousDayViewShift(t *testing.T) {
	location, err := time.LoadLocation(manilaLocationName)
	if err != nil {
		t.Fatalf("load Manila location: %v", err)
	}
	businessDate := time.Date(2026, time.July, 21, 0, 0, 0, 0, location)

	start, end, err := yesterdayWindowUTC(businessDate)
	if err != nil {
		t.Fatalf("yesterdayWindowUTC returned error: %v", err)
	}

	wantStart := time.Date(2026, time.July, 20, 5, 0, 0, 0, time.UTC) // 1:00 PM Manila
	wantEnd := time.Date(2026, time.July, 20, 20, 0, 0, 0, time.UTC)  // 4:00 AM Manila next day
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("window = [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
}
