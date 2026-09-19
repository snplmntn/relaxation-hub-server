package service

import "math"

const regularShiftMinutes = 8 * 60
const overtimeGraceMinutes = 14

type DailyRatePayResult struct {
	RegularMinutes  int
	OvertimeMinutes int
	GrossCents      int64
}

func CalculateDailyRatePay(dailyRateCents int64, overtimeMultiplier float64, workedMinutes int) DailyRatePayResult {
	if dailyRateCents <= 0 || workedMinutes <= 0 {
		return DailyRatePayResult{}
	}
	overtimeMultiplierUnits := fixedMultiplierUnits(overtimeMultiplier)

	if workedMinutes < regularShiftMinutes {
		return DailyRatePayResult{
			RegularMinutes: workedMinutes,
			GrossCents:     roundDiv(dailyRateCents*int64(workedMinutes), regularShiftMinutes),
		}
	}

	overtimeMinutes := 0
	if workedMinutes > regularShiftMinutes+overtimeGraceMinutes {
		overtimeMinutes = workedMinutes - regularShiftMinutes
	}

	amount := dailyRateCents
	if overtimeMinutes > 0 {
		overtimePay := roundDiv(
			dailyRateCents*overtimeMultiplierUnits*int64(overtimeMinutes),
			regularShiftMinutes*10000,
		)
		amount += overtimePay
	}

	return DailyRatePayResult{
		RegularMinutes:  regularShiftMinutes,
		OvertimeMinutes: overtimeMinutes,
		GrossCents:      amount,
	}
}

func fixedMultiplierUnits(multiplier float64) int64 {
	if multiplier <= 0 || math.IsNaN(multiplier) {
		return 0
	}
	return int64(math.Round(multiplier * 10000))
}

func roundDiv(numerator, denominator int64) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (numerator + denominator/2) / denominator
}
