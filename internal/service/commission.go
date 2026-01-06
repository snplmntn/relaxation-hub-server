package service

// CalculateCommission computes therapist earnings based on service commission and booking duration.
// The logic is: commission scales linearly with duration relative to the service's base duration.
//
// Parameters:
//   - baseCommission: therapist_commission from the service (payment for base duration).
//   - basePrice: base_price from the service (cost for base duration).
//   - baseDuration: service's default duration in minutes.
//   - totalDuration: booking's actual duration in minutes (including extensions).
//
// Formula: Commission = (totalDuration / baseDuration) * baseCommission
// Equivalently: BaseCommission + (extra time cost) * (baseCommission / basePrice)
func CalculateCommission(baseCommission, basePrice float64, baseDuration, totalDuration int) float64 {
	if baseDuration <= 0 || baseCommission <= 0 {
		return 0
	}
	// Linear scaling: full duration / base duration * base commission
	return (float64(totalDuration) / float64(baseDuration)) * baseCommission
}
