package model

import "time"

// RiderWallet represents a rider's earnings wallet
type RiderWallet struct {
	RiderID              int64     `json:"rider_id" db:"rider_id"`
	BalanceCents         int       `json:"balance_cents" db:"balance_cents"`
	TotalEarnedCents     int       `json:"total_earned_cents" db:"total_earned_cents"`
	TotalWithdrawnCents  int       `json:"total_withdrawn_cents" db:"total_withdrawn_cents"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// RiderTransaction represents a wallet transaction
type RiderTransaction struct {
	TransactionID   int        `json:"transaction_id" db:"transaction_id"`
	RiderID         int64      `json:"rider_id" db:"rider_id"`
	TransactionType string     `json:"transaction_type" db:"transaction_type"` // ride_earning, payout, adjustment, bonus
	AmountCents     int        `json:"amount_cents" db:"amount_cents"`
	RideID          *int64     `json:"ride_id,omitempty" db:"ride_id"`
	PayoutMethodID  *int       `json:"payout_method_id,omitempty" db:"payout_method_id"`
	Status          string     `json:"status" db:"status"` // pending, completed, failed
	Description     *string    `json:"description,omitempty" db:"description"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// RiderPayoutMethod represents a stored payout destination for a rider
type RiderPayoutMethod struct {
	ID            int       `json:"id" db:"id"`
	RiderID       int64     `json:"rider_id" db:"rider_id"`
	MethodType    string    `json:"method_type" db:"method_type"` // bank, gcash, paymaya, grabpay
	ProviderName  string    `json:"provider_name" db:"provider_name"`
	AccountNumber string    `json:"account_number" db:"account_number"`
	AccountName   string    `json:"account_name" db:"account_name"`
	IsDefault     bool      `json:"is_default" db:"is_default"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// RiderPerformanceMetrics tracks rider quality and acceptance rates
type RiderPerformanceMetrics struct {
	RiderID              int64      `json:"rider_id" db:"rider_id"`
	TotalOffersReceived  int        `json:"total_offers_received" db:"total_offers_received"`
	TotalRidesAccepted   int        `json:"total_rides_accepted" db:"total_rides_accepted"`
	TotalRidesCompleted  int        `json:"total_rides_completed" db:"total_rides_completed"`
	TotalRidesCancelled  int        `json:"total_rides_cancelled" db:"total_rides_cancelled"`
	AcceptanceRate       float64    `json:"acceptance_rate" db:"acceptance_rate"`
	CompletionRate       float64    `json:"completion_rate" db:"completion_rate"`
	AverageRating        *float64   `json:"average_rating,omitempty" db:"average_rating"`
	TotalRatings         int        `json:"total_ratings" db:"total_ratings"`
	RatingSum            int        `json:"rating_sum" db:"rating_sum"`
	TodayEarnedCents     int        `json:"today_earned_cents" db:"-"` // Calculated field, not in DB table
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
}

// RiderEmergencyContact represents an emergency contact for safety features
type RiderEmergencyContact struct {
	ContactID   int       `json:"contact_id" db:"contact_id"`
	RiderID     int64     `json:"rider_id" db:"rider_id"`
	FullName    string    `json:"full_name" db:"full_name"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Relationship *string  `json:"relationship,omitempty" db:"relationship"`
	IsPrimary   bool      `json:"is_primary" db:"is_primary"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Request/Response types

type RiderPayoutRequest struct {
	AmountCents    int `json:"amount_cents" binding:"required,min=10000"` // Minimum 100 PHP
	PayoutMethodID int `json:"payout_method_id" binding:"required"`
}

type WalletResponse struct {
	Balance          float64 `json:"balance"`           // In PHP
	TotalEarned      float64 `json:"total_earned"`
	TotalWithdrawn   float64 `json:"total_withdrawn"`
	BalanceCents     int     `json:"balance_cents"`
	TotalEarnedCents int     `json:"total_earned_cents"`
	TotalWithdrawnCents int  `json:"total_withdrawn_cents"`
}

type TransactionResponse struct {
	TransactionID   int        `json:"transaction_id"`
	Type            string     `json:"type"`
	Amount          float64    `json:"amount"`           // In PHP
	AmountCents     int        `json:"amount_cents"`
	RideID          *int64     `json:"ride_id,omitempty"`
	PayoutMethodID  *int       `json:"payout_method_id,omitempty"`
	Status          string     `json:"status"`
	Description     *string    `json:"description,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type PerformanceResponse struct {
	AcceptanceRate  float64  `json:"acceptance_rate"`
	CompletionRate  float64  `json:"completion_rate"`
	AverageRating   *float64 `json:"average_rating,omitempty"`
	TotalRides      int      `json:"total_rides"`
	CompletedRides  int      `json:"completed_rides"`
	TodayEarned     float64  `json:"today_earned"`
}
