package service

import "math"

// CandidateScore represents a therapist's calculated ranking score.
type CandidateScore struct {
	TherapistID int64
	Score       float64
	FairScore   float64
	DistScore   float64
}

// WeightFair is the weight for fairness scoring (higher = prioritize struggling therapists).
const WeightFair = 0.6

// WeightDist is the weight for distance scoring (higher = prioritize closer therapists).
const WeightDist = 0.4

// CalculateCandidateScore computes a weighted score for ranking therapists.
// Higher score = Better candidate.
//
// Formula: Score = (WeightFair * FairScore) + (WeightDist * DistScore)
//
// FairScore: 100 if bookingCount=0, decays as bookings increase relative to average.
// DistScore: 100 if distance<1km, 0 if distance>10km, linear decay in between.
func CalculateCandidateScore(
	bookingCount int,
	avgBookingCount float64,
	distanceKm float64,
) CandidateScore {
	// Fairness Score (0-100)
	// Therapists with 0 bookings get 100, those at or above average get 0.
	fairScore := 100.0
	if avgBookingCount > 0 {
		ratio := float64(bookingCount) / avgBookingCount
		fairScore = math.Max(0, 100-(ratio*100))
	}

	// Distance Score (0-100)
	// <1km = 100, >10km = 0, linear decay in between.
	distScore := 100.0
	if distanceKm < 0 {
		// Invalid or unknown distance: neutral score
		distScore = 50.0
	} else if distanceKm >= 10 {
		distScore = 0
	} else if distanceKm > 1 {
		// Linear decay: 1km=100, 10km=0 => slope = -100/9 ≈ -11.1
		distScore = 100 - ((distanceKm - 1) * (100.0 / 9.0))
	}

	totalScore := (WeightFair * fairScore) + (WeightDist * distScore)

	return CandidateScore{
		Score:     totalScore,
		FairScore: fairScore,
		DistScore: distScore,
	}
}
