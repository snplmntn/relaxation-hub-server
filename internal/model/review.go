package model

import "time"

// Review maps to the reviews table.
type Review struct {
	ReviewID        int64      `db:"review_id" json:"review_id"`
	BookingID       int64      `db:"booking_id" json:"booking_id"`
	ClientID        int64      `db:"client_id" json:"client_id"`
	TherapistID     int64      `db:"therapist_id" json:"therapist_id"`
	ServiceID       int64      `db:"service_id" json:"service_id"`
	TherapistRating int        `db:"therapist_rating" json:"therapist_rating"`
	TherapistReview string     `db:"therapist_review" json:"therapist_review"`
	ServiceRating   int        `db:"service_rating" json:"service_rating"`
	ServiceReview   string     `db:"service_review" json:"service_review"`
	PlatformRating  int        `db:"platform_rating" json:"platform_rating"`
	PlatformReview  string     `db:"platform_review" json:"platform_review"`
	DeletedAt       *time.Time `db:"deleted_at" json:"-"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateReviewRequest captures review input.
type CreateReviewRequest struct {
	BookingID       int64  `json:"booking_id"`
	TherapistRating int    `json:"therapist_rating"`
	TherapistReview string `json:"therapist_review"`
	ServiceRating   int    `json:"service_rating"`
	ServiceReview   string `json:"service_review"`
	PlatformRating  int    `json:"platform_rating"`
	PlatformReview  string `json:"platform_review"`
}

// ReviewResponse to clients.
type ReviewResponse struct {
	ReviewID        int64     `json:"review_id"`
	BookingID       int64     `json:"booking_id"`
	ClientID        int64     `json:"client_id"`
	TherapistID     int64     `json:"therapist_id"`
	ServiceID       int64     `json:"service_id"`
	Service         *Service  `json:"service,omitempty"`
	TherapistRating int       `json:"therapist_rating"`
	TherapistReview string    `json:"therapist_review"`
	ServiceRating   int       `json:"service_rating"`
	ServiceReview   string    `json:"service_review"`
	PlatformRating  int       `json:"platform_rating"`
	PlatformReview  string    `json:"platform_review"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
