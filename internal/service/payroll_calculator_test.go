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

func TestCalculateDailyRatePay_ZeroDailyRatePaysZero(t *testing.T) {
	got := CalculateDailyRatePay(0, 1.25, 495)
	if got.RegularMinutes != 0 || got.OvertimeMinutes != 0 || got.GrossCents != 0 {
		t.Fatalf("zero daily rate should pay zero, got %+v", got)
	}
}

func TestCalculateDailyRatePay_ZeroOrNegativeMultiplierPaysNoOvertimePremium(t *testing.T) {
	tests := []struct {
		name               string
		overtimeMultiplier float64
	}{
		{name: "zero multiplier", overtimeMultiplier: 0},
		{name: "negative multiplier", overtimeMultiplier: -1.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDailyRatePay(75000, tt.overtimeMultiplier, 495)
			if got.RegularMinutes != 480 {
				t.Fatalf("regular minutes = %d, want 480", got.RegularMinutes)
			}
			if got.OvertimeMinutes != 15 {
				t.Fatalf("overtime minutes = %d, want 15", got.OvertimeMinutes)
			}
			if got.GrossCents != 75000 {
				t.Fatalf("gross cents = %d, want 75000", got.GrossCents)
			}
		})
	}
}

func TestCalculateDailyRatePay_RoundsHalfCentProrationUp(t *testing.T) {
	got := CalculateDailyRatePay(1100, 1.25, 108)
	if got.RegularMinutes != 108 {
		t.Fatalf("regular minutes = %d, want 108", got.RegularMinutes)
	}
	if got.OvertimeMinutes != 0 {
		t.Fatalf("overtime minutes = %d, want 0", got.OvertimeMinutes)
	}
	if got.GrossCents != 248 {
		t.Fatalf("gross cents = %d, want 248", got.GrossCents)
	}
}

func TestCalculateDailyRatePay_NonDefaultMultiplier(t *testing.T) {
	got := CalculateDailyRatePay(80000, 1.5, 510)
	if got.RegularMinutes != 480 {
		t.Fatalf("regular minutes = %d, want 480", got.RegularMinutes)
	}
	if got.OvertimeMinutes != 30 {
		t.Fatalf("overtime minutes = %d, want 30", got.OvertimeMinutes)
	}
	if got.GrossCents != 87500 {
		t.Fatalf("gross cents = %d, want 87500", got.GrossCents)
	}
}

func TestRoundDiv_RoundsPositiveValuesHalfUp(t *testing.T) {
	tests := []struct {
		name        string
		numerator   int64
		denominator int64
		want        int64
	}{
		{name: "below half rounds down", numerator: 2474, denominator: 10, want: 247},
		{name: "exact half rounds up", numerator: 2475, denominator: 10, want: 248},
		{name: "above half rounds up", numerator: 2476, denominator: 10, want: 248},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roundDiv(tt.numerator, tt.denominator); got != tt.want {
				t.Fatalf("roundDiv(%d, %d) = %d, want %d", tt.numerator, tt.denominator, got, tt.want)
			}
		})
	}
}
