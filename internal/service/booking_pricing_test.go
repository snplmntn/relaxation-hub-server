package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// Baso / Ventosa is PHP 1,099 for 120 minutes in the live catalog. A shorter
// session is billed per minute off that rate — it used to be billed the full
// PHP 1,099, so a 1.5-hour booking charged the client for two hours while the
// therapist was only paid for 1.5.
func TestBookingPriceForDuration_PerMinuteInBothDirections(t *testing.T) {
	baso := &resolvedBookingServices{TotalBasePrice: 1099, TotalBaseDuration: 120}

	for _, tc := range []struct {
		name     string
		minutes  int
		expected float64
	}{
		{"below base duration", 90, 824.25},
		{"at base duration", 120, 1099},
		{"above base duration", 150, 1373.75},
	} {
		if got := bookingPriceForDuration(baso, tc.minutes); got != tc.expected {
			t.Errorf("%s: expected %.2f for %d minutes, got %.2f", tc.name, tc.expected, tc.minutes, got)
		}
	}

	// A rate that does not divide cleanly must not leave float noise behind.
	signature := &resolvedBookingServices{TotalBasePrice: 1000, TotalBaseDuration: 60}
	if got := bookingPriceForDuration(signature, 60); got != 1000 {
		t.Errorf("expected exactly 1000 at the base duration, got %v", got)
	}
	if got := bookingPriceForDuration(signature, 90); got != 1500 {
		t.Errorf("expected exactly 1500 for 90 minutes, got %v", got)
	}
}

func TestBookingPriceForDuration_UsesEachServiceAllocation(t *testing.T) {
	deepTissueMinutes := 120
	signatureMinutes := 60
	selection := &resolvedBookingServices{
		TotalBasePrice:    1148,
		TotalBaseDuration: 120,
		Items: []model.BookingService{
			{
				ServiceID:                1,
				PriceSnapshot:            649,
				DurationSnapshot:         60,
				AllocatedDurationMinutes: &deepTissueMinutes,
			},
			{
				ServiceID:                2,
				PriceSnapshot:            499,
				DurationSnapshot:         60,
				AllocatedDurationMinutes: &signatureMinutes,
			},
		},
	}

	rawTotal := bookingPriceForDuration(selection, 180)
	if rawTotal != 1797 {
		t.Fatalf("expected allocated service total 1797, got %.2f", rawTotal)
	}

	twentyPct := 20
	discount := promoDiscountFor(&model.Promotion{DiscountPct: &twentyPct}, rawTotal)
	finalTotal := computeFinal(&rawTotal, discount)
	if discount == nil || roundCurrency(*discount) != 359.40 {
		t.Fatalf("expected voucher discount 359.40, got %v", discount)
	}
	if finalTotal == nil || roundCurrency(*finalTotal) != 1437.60 {
		t.Fatalf("expected voucher-adjusted total 1437.60, got %v", finalTotal)
	}
}

func TestRepriceAttachedVoucher(t *testing.T) {
	ctx := context.Background()
	promoID := int64(7)
	twentyPct := 20

	// A percentage voucher follows the price, so a 30-minute extension stays
	// covered without staff re-entering the code.
	promoRepo := &MockPromoRepository{PromoByID: &model.Promotion{PromoID: promoID, Code: "SAVE20", DiscountPct: &twentyPct}}
	s := NewBookingService(nil, promoRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	extended := 1099.0
	booking := &model.Booking{PromoID: &promoID, RawTotal: &extended, Discount: float64Ptr(164.85), FinalTotal: float64Ptr(659.40)}
	s.repriceAttachedVoucher(ctx, booking)
	if *booking.Discount != 219.80 || *booking.FinalTotal != 879.20 {
		t.Errorf("expected 219.80 off 1099 leaving 879.20, got %.2f off leaving %.2f", *booking.Discount, *booking.FinalTotal)
	}

	// A fixed-peso voucher is worth the same whatever the booking costs.
	fixed := 150.0
	promoRepo.PromoByID = &model.Promotion{PromoID: promoID, Code: "LESS150", DiscountAmount: &fixed}
	booking = &model.Booking{PromoID: &promoID, RawTotal: &extended, Discount: float64Ptr(150), FinalTotal: float64Ptr(674.25)}
	s.repriceAttachedVoucher(ctx, booking)
	if *booking.Discount != 150 || *booking.FinalTotal != 949 {
		t.Errorf("expected 150 off 1099 leaving 949, got %.2f off leaving %.2f", *booking.Discount, *booking.FinalTotal)
	}

	// VIP and manual discounts carry no promo id and must be left alone.
	booking = &model.Booking{RawTotal: &extended, Discount: float64Ptr(82.42), FinalTotal: float64Ptr(1016.58)}
	s.repriceAttachedVoucher(ctx, booking)
	if *booking.Discount != 82.42 || *booking.FinalTotal != 1016.58 {
		t.Errorf("a booking with no voucher must keep its discount, got %.2f off leaving %.2f", *booking.Discount, *booking.FinalTotal)
	}
}
