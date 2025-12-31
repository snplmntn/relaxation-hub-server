package model

import "time"

// Promotion represents the promotions table.
type Promotion struct {
	PromoID     int64      `db:"promo_id" json:"promo_id"`
	Code        string     `db:"code" json:"code"`
	DiscountPct *int       `db:"discount_percentage" json:"discount_percent,omitempty"`
	DiscountAmount *float64 `db:"discount_amount" json:"discount_amount,omitempty"`
	ValidFrom   *time.Time `db:"valid_from" json:"valid_from,omitempty"`
	ValidUntil  *time.Time `db:"valid_until" json:"valid_until,omitempty"`
	UsageLimit  int        `db:"max_uses" json:"max_uses"`
	CurrentUses int        `db:"current_uses" json:"current_uses"`
	DaysOfWeek  []int32    `db:"days_of_week" json:"days_of_week,omitempty"`
	StartTime   *time.Time `db:"start_time" json:"start_time,omitempty"`
	EndTime     *time.Time `db:"end_time" json:"end_time,omitempty"`
	DeletedAt   *time.Time `db:"deleted_at" json:"-"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// CreatePromotionRequest is used to create a promo.
type CreatePromotionRequest struct {
	Code        string     `json:"code"`
	DiscountPct int        `json:"discount_percent"`
	DiscountAmount *float64 `json:"discount_amount"`
	ValidFrom   *string    `json:"valid_from"`
	ValidUntil  *string `json:"valid_until"`
	UsageLimit  *int    `json:"max_uses"`
	DaysOfWeek  []int32 `json:"days_of_week"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
}

// PromotionResponse is returned to clients.
type PromotionResponse struct {
	PromoID     int64      `json:"promo_id"`
	Code        string     `json:"code"`
	DiscountPct *int       `json:"discount_percent,omitempty"`
	DiscountAmount *float64 `json:"discount_amount,omitempty"`
	ValidFrom   *time.Time `json:"valid_from,omitempty"`
	ValidUntil  *time.Time `json:"valid_until,omitempty"`
	UsageLimit  int        `json:"max_uses"`
	CurrentUses int        `json:"current_uses"`
	DaysOfWeek  []int32    `json:"days_of_week,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
