package service

import (
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestComputeOccurrencesWeeklyLandsOnSelectedWeekday(t *testing.T) {
	// Start on Monday 2026-06-01; repeat weekly on Friday (5) at 14:00.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, manilaLocation)
	rec := &model.RecurringBooking{
		Frequency:     "weekly",
		IntervalValue: 1,
		DaysOfWeek:    []int{5}, // Friday
		TimeOfDay:     "14:00",
		StartDate:     start,
	}

	until := start.AddDate(0, 0, 21) // 3 weeks
	occ := computeOccurrences(rec, start, until)

	if len(occ) == 0 {
		t.Fatal("expected at least one occurrence, got none")
	}
	for _, o := range occ {
		om := o.In(manilaLocation)
		if om.Weekday() != time.Friday {
			t.Errorf("occurrence %s is %s, want Friday", om.Format(time.RFC3339), om.Weekday())
		}
		if om.Hour() != 14 || om.Minute() != 0 {
			t.Errorf("occurrence %s time = %02d:%02d, want 14:00", om.Format(time.RFC3339), om.Hour(), om.Minute())
		}
	}

	// First Friday on/after Mon 2026-06-01 is 2026-06-05.
	first := occ[0].In(manilaLocation)
	if first.Year() != 2026 || first.Month() != time.June || first.Day() != 5 {
		t.Errorf("first occurrence = %s, want 2026-06-05", first.Format(time.RFC3339))
	}
}

func TestComputeOccurrencesWeeklyMultipleDays(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, manilaLocation) // Monday
	rec := &model.RecurringBooking{
		Frequency:     "weekly",
		IntervalValue: 1,
		DaysOfWeek:    []int{1, 3}, // Mon, Wed
		TimeOfDay:     "09:30",
		StartDate:     start,
	}
	until := start.AddDate(0, 0, 7) // one week
	occ := computeOccurrences(rec, start, until)

	days := map[time.Weekday]bool{}
	for _, o := range occ {
		days[o.In(manilaLocation).Weekday()] = true
	}
	if !days[time.Monday] || !days[time.Wednesday] {
		t.Errorf("expected Monday and Wednesday occurrences, got %v", days)
	}
}

func TestComputeOccurrencesDaily(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, manilaLocation)
	rec := &model.RecurringBooking{
		Frequency:     "daily",
		IntervalValue: 1,
		TimeOfDay:     "08:00",
		StartDate:     start,
	}
	until := start.AddDate(0, 0, 4).Add(23 * time.Hour) // through end of day 5
	occ := computeOccurrences(rec, start, until)
	if len(occ) != 5 {
		t.Fatalf("expected 5 daily occurrences, got %d", len(occ))
	}
	for i, o := range occ {
		om := o.In(manilaLocation)
		if om.Day() != 1+i {
			t.Errorf("occurrence %d on day %d, want %d", i, om.Day(), 1+i)
		}
	}
}

func TestComputeOccurrencesMonthlyClampsShortMonth(t *testing.T) {
	// Day 31 each month: February must clamp to the last day.
	start := time.Date(2026, 1, 31, 0, 0, 0, 0, manilaLocation)
	dom := 31
	rec := &model.RecurringBooking{
		Frequency:     "monthly",
		IntervalValue: 1,
		DayOfMonth:    &dom,
		TimeOfDay:     "10:00",
		StartDate:     start,
	}
	until := time.Date(2026, 3, 31, 23, 59, 0, 0, manilaLocation)
	occ := computeOccurrences(rec, start, until)

	var feb *time.Time
	for i := range occ {
		om := occ[i].In(manilaLocation)
		if om.Month() == time.February {
			feb = &occ[i]
		}
	}
	if feb == nil {
		t.Fatal("expected a February occurrence")
	}
	fm := feb.In(manilaLocation)
	if fm.Month() != time.February || fm.Day() != 28 {
		t.Errorf("February occurrence = %s, want clamped to Feb 28", fm.Format(time.RFC3339))
	}
}

// TestComputeOccurrencesWorkerPathUTCStartDate simulates the worker reloading a
// series from Postgres, where pgx returns start_date as UTC midnight. The result
// must still land on the selected local weekday/time in Manila.
func TestComputeOccurrencesWorkerPathUTCStartDate(t *testing.T) {
	// start_date 2026-05-31 as UTC midnight (how pgx scans a PG date).
	startUTC := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	rec := &model.RecurringBooking{
		Frequency:     "weekly",
		IntervalValue: 1,
		DaysOfWeek:    []int{0}, // Sunday
		TimeOfDay:     "21:45",
		StartDate:     startUTC,
	}
	// `from` also arrives as a UTC instant in the worker path.
	from := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	until := from.AddDate(0, 0, 14)
	occ := computeOccurrences(rec, from, until)

	if len(occ) == 0 {
		t.Fatal("expected occurrences, got none")
	}
	first := occ[0].In(manilaLocation)
	// Sunday 2026-05-31 at 21:45 Manila — NOT Monday 05:45 (the UTC-shifted bug).
	if first.Weekday() != time.Sunday {
		t.Errorf("first occurrence weekday = %s, want Sunday", first.Weekday())
	}
	if first.Hour() != 21 || first.Minute() != 45 {
		t.Errorf("first occurrence time = %02d:%02d, want 21:45 Manila", first.Hour(), first.Minute())
	}
	if first.Day() != 31 || first.Month() != time.May {
		t.Errorf("first occurrence date = %s, want 2026-05-31", first.Format("2006-01-02"))
	}
}

func TestComputeOccurrencesRespectsEndDate(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, manilaLocation)
	end := time.Date(2026, 6, 3, 0, 0, 0, 0, manilaLocation)
	rec := &model.RecurringBooking{
		Frequency:     "daily",
		IntervalValue: 1,
		TimeOfDay:     "08:00",
		StartDate:     start,
		EndDate:       &end,
	}
	until := start.AddDate(0, 0, 30) // horizon well past end
	occ := computeOccurrences(rec, start, until)
	// Jun 1, 2, 3 → 3 occurrences (end date inclusive).
	if len(occ) != 3 {
		t.Fatalf("expected 3 occurrences up to end date, got %d", len(occ))
	}
}
