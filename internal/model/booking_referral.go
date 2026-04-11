package model

import "time"

type BookingReferral struct {
	BookingID       int64     `db:"booking_id" json:"booking_id"`
	Source          string    `db:"source" json:"source"`
	OtherNotes      *string   `db:"other_notes" json:"other_notes,omitempty"`
	CreatedByUserID int64     `db:"created_by_user_id" json:"created_by_user_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

type BookingReferralSummaryTotal struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

type BookingReferralSummaryPoint struct {
	PeriodStart time.Time `json:"period_start"`
	Source      string    `json:"source"`
	Count       int64     `json:"count"`
}
