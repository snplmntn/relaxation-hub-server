package service

import "testing"

func TestAllocateGroupTipsPreservesCents(t *testing.T) {
	got := allocateGroupTips(3, 100)
	want := []float64{33.34, 33.33, 33.33}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("allocation %d = %.2f, want %.2f", i, got[i], want[i])
		}
	}
}

func TestNormalizeBookingTip(t *testing.T) {
	got, err := normalizeBookingTip(99.999)
	if err != nil || got != 100 {
		t.Fatalf("normalizeBookingTip() = %.2f, %v; want 100.00, nil", got, err)
	}
	for _, invalid := range []float64{-1, 10000.01} {
		if _, err := normalizeBookingTip(invalid); err == nil {
			t.Fatalf("normalizeBookingTip(%v) expected an error", invalid)
		}
	}
}

func TestFinalTotalWithTipAddsTipAfterDiscount(t *testing.T) {
	raw, discount := 1000.0, 100.0
	got := finalTotalWithTip(&raw, &discount, 75)
	if got == nil || *got != 975 {
		t.Fatalf("finalTotalWithTip() = %v, want 975", got)
	}
}
