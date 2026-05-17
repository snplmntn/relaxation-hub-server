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
	if overtimeMultiplier < 0 {
		overtimeMultiplier = 0
	}
	if workedMinutes < regularShiftMinutes {
		amount := (float64(dailyRateCents) / float64(regularShiftMinutes)) * float64(workedMinutes)
		return DailyRatePayResult{
			RegularMinutes: workedMinutes,
			GrossCents:     int64(math.Round(amount)),
		}
	}

	overtimeMinutes := 0
	if workedMinutes > regularShiftMinutes+overtimeGraceMinutes {
		overtimeMinutes = workedMinutes - regularShiftMinutes
	}

	amount := float64(dailyRateCents)
	if overtimeMinutes > 0 {
		overtimeMinuteRate := ((float64(dailyRateCents) / 8.0) * overtimeMultiplier) / 60.0
		amount += overtimeMinuteRate * float64(overtimeMinutes)
	}

	return DailyRatePayResult{
		RegularMinutes:  regularShiftMinutes,
		OvertimeMinutes: overtimeMinutes,
		GrossCents:      int64(math.Round(amount)),
	}
}
