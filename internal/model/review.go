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

// ClientReview maps to the client_reviews table.
type ClientReview struct {
	ClientReviewID int64      `db:"client_review_id" json:"client_review_id"`
	BookingID      int64      `db:"booking_id" json:"booking_id"`
	TherapistID    int64      `db:"therapist_id" json:"therapist_id"`
	ClientID       int64      `db:"client_id" json:"client_id"`
	ClientRating   int        `db:"client_rating" json:"client_rating"`
	ClientReview   string     `db:"client_review" json:"client_review"`
	DeletedAt      *time.Time `db:"deleted_at" json:"-"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
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

// CreateClientReviewRequest captures therapist feedback about a client.
type CreateClientReviewRequest struct {
	BookingID    int64  `json:"booking_id"`
	ClientRating int    `json:"client_rating"`
	ClientReview string `json:"client_review"`
}

// ReviewResponse to clients.
type ReviewResponse struct {
	ReviewID        int64      `json:"review_id"`
	BookingID       int64      `json:"booking_id"`
	ClientID        int64      `json:"client_id"`
	TherapistID     int64      `json:"therapist_id"`
	ServiceID       int64      `json:"service_id"`
	Service         *Service   `json:"service,omitempty"`
	BookingDate     *time.Time `json:"booking_date,omitempty"`
	TherapistName   string     `json:"therapist_name,omitempty"`
	TherapistPhoto  string     `json:"therapist_photo,omitempty"`
	ClientName      string     `json:"client_name,omitempty"`
	ClientPhoto     string     `json:"client_photo,omitempty"`
	TherapistRating int        `json:"therapist_rating"`
	TherapistReview string     `json:"therapist_review"`
	ServiceRating   int        `json:"service_rating"`
	ServiceReview   string     `json:"service_review"`
	PlatformRating  int        `json:"platform_rating"`
	PlatformReview  string     `json:"platform_review"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ClientReviewResponse represents API output for therapist-authored reviews of clients.
type ClientReviewResponse struct {
	ClientReviewID int64     `json:"client_review_id"`
	BookingID      int64     `json:"booking_id"`
	TherapistID    int64     `json:"therapist_id"`
	ClientID       int64     `json:"client_id"`
	ClientRating   int       `json:"client_rating"`
	ClientReview   string    `json:"client_review"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PaginatedReviewsResponse wraps a list of reviews with pagination metadata.
type PaginatedReviewsResponse struct {
	Reviews    []ReviewResponse `json:"reviews"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
	HasMore    bool             `json:"has_more"`
}
