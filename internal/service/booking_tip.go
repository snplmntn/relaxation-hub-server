package service

import (
	"math"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

const maxBookingTipAmount = 10000.0

func normalizeBookingTip(amount float64) (float64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > maxBookingTipAmount {
		return 0, NewValidationError(
			"invalid_tip_amount",
			"Tip amount must be between ₱0 and ₱10,000.",
			map[string]string{"tip_amount": "must be between 0 and 10000"},
		)
	}
	return roundCurrency(amount), nil
}

func finalTotalWithTip(raw, discount *float64, tip float64) *float64 {
	base := computeFinal(raw, discount)
	if base == nil {
		return nil
	}
	total := roundCurrency(*base + tip)
	return &total
}

func earningsWithTip(earnings *float64, tip float64) *float64 {
	if earnings == nil && tip == 0 {
		return nil
	}
	total := tip
	if earnings != nil {
		total += *earnings
	}
	total = roundCurrency(total)
	return &total
}

// allocateGroupTips splits the gratuity evenly, preserving the exact amount
// down to the cent by assigning any remainder to the first bookings.
func allocateGroupTips(count int, total float64) []float64 {
	result := make([]float64, count)
	if count == 0 || total <= 0 {
		return result
	}
	totalCents := int(math.Round(total * 100))
	base := totalCents / count
	remainder := totalCents % count
	for i := range result {
		cents := base
		if i < remainder {
			cents++
		}
		result[i] = float64(cents) / 100
	}
	return result
}

func addBookingTipToEarnings(booking *model.Booking, earnings *float64) *float64 {
	if booking == nil {
		return earnings
	}
	return earningsWithTip(earnings, booking.TipAmount)
}
