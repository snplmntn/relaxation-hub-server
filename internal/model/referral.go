package model

import "time"

// Referral represents the referrals table.
type Referral struct {
	ReferralID   int64      `db:"referral_id" json:"referral_id"`
	ReferrerID   int64      `db:"referrer_id" json:"referrer_id"`
	ReferredID   int64      `db:"referred_id" json:"referred_id"`
	ReferralCode string     `db:"referral_code" json:"referral_code"`
	Status       string     `db:"status" json:"status"`
	RewardEarned bool       `db:"reward_earned" json:"reward_earned"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	CompletedAt  *time.Time `db:"completed_at" json:"completed_at,omitempty"`
}

// ReferralReward represents the referral_rewards table.
type ReferralReward struct {
	RewardID     int64      `db:"reward_id" json:"reward_id"`
	ReferralID   int64      `db:"referral_id" json:"referral_id"`
	UserID       int64      `db:"user_id" json:"user_id"`
	RewardType   string     `db:"reward_type" json:"reward_type"`
	RewardAmount float64    `db:"reward_amount" json:"reward_amount"`
	Status       string     `db:"status" json:"status"`
	ExpiresAt    *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	RedeemedAt   *time.Time `db:"redeemed_at" json:"redeemed_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}

// CreateReferralRequest for generating referral.
type CreateReferralRequest struct {
	ReferredID int64 `json:"referred_id"`
}

// ReferralResponse to clients.
type ReferralResponse struct {
	ReferralID   int64      `json:"referral_id"`
	ReferrerID   int64      `json:"referrer_id"`
	ReferredID   int64      `json:"referred_id"`
	ReferralCode string     `json:"referral_code"`
	Status       string     `json:"status"`
	RewardEarned bool       `json:"reward_earned"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// ReferralRewardResponse to clients.
type ReferralRewardResponse struct {
	RewardID     int64      `json:"reward_id"`
	ReferralID   int64      `json:"referral_id"`
	UserID       int64      `json:"user_id"`
	RewardType   string     `json:"reward_type"`
	RewardAmount float64    `json:"reward_amount"`
	Status       string     `json:"status"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	RedeemedAt   *time.Time `json:"redeemed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ReferralRewardTotals struct {
	TotalAmount    float64 `json:"total_amount"`
	PendingAmount  float64 `json:"pending_amount"`
	RedeemedAmount float64 `json:"redeemed_amount"`
	PendingCount   int     `json:"pending_count"`
	RedeemedCount  int     `json:"redeemed_count"`
}

type ReferralSummaryResponse struct {
	TotalReferrals      int                      `json:"total_referrals"`
	SuccessfulReferrals int                      `json:"successful_referrals"`
	PendingReferrals    int                      `json:"pending_referrals"`
	Rewards             []ReferralRewardResponse `json:"rewards"`
	RewardTotals        ReferralRewardTotals     `json:"reward_totals"`
}

type ReferralMyCodeResponse struct {
	ReferralCode string `json:"referral_code"`
}
