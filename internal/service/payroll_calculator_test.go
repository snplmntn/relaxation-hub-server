package service

import "testing"

func TestCalculateDailyRatePay_ProrationAndOvertime(t *testing.T) {
	tests := []struct {
		name           string
		workedMinutes  int
		wantRegular    int
		wantOvertime   int
		wantGrossCents int64
	}{
		{name: "seven and a half hours prorates by minute", workedMinutes: 450, wantRegular: 450, wantOvertime: 0, wantGrossCents: 70313},
		{name: "exactly eight hours pays full day", workedMinutes: 480, wantRegular: 480, wantOvertime: 0, wantGrossCents: 75000},
		{name: "eight hours fourteen minutes has no overtime", workedMinutes: 494, wantRegular: 480, wantOvertime: 0, wantGrossCents: 75000},
		{name: "eight hours fifteen minutes pays overtime minutes", workedMinutes: 495, wantRegular: 480, wantOvertime: 15, wantGrossCents: 77930},
		{name: "nine hours pays sixty overtime minutes", workedMinutes: 540, wantRegular: 480, wantOvertime: 60, wantGrossCents: 86719},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDailyRatePay(75000, 1.25, tt.workedMinutes)
			if got.RegularMinutes != tt.wantRegular {
				t.Fatalf("regular minutes = %d, want %d", got.RegularMinutes, tt.wantRegular)
			}
			if got.OvertimeMinutes != tt.wantOvertime {
				t.Fatalf("overtime minutes = %d, want %d", got.OvertimeMinutes, tt.wantOvertime)
			}
			if got.GrossCents != tt.wantGrossCents {
				t.Fatalf("gross cents = %d, want %d", got.GrossCents, tt.wantGrossCents)
			}
		})
	}
}

func TestCalculateDailyRatePay_RejectsNegativeMinutes(t *testing.T) {
	got := CalculateDailyRatePay(75000, 1.25, -10)
	if got.RegularMinutes != 0 || got.OvertimeMinutes != 0 || got.GrossCents != 0 {
		t.Fatalf("negative worked minutes should pay zero, got %+v", got)
	}
}
