package model

import "time"

// TherapistProfile represents the therapist_profiles table.
type TherapistProfile struct {
	TherapistID         int64     `db:"therapist_id" json:"therapist_id"`
	Bio                 *string   `db:"bio" json:"bio,omitempty"`
	Specialization      *string   `db:"specialization" json:"specialization,omitempty"`
	YearsExperience     *int      `db:"years_experience" json:"years_experience,omitempty"`
	Gender              string    `json:"gender,omitempty"` // From users table, not in therapist_profiles
	PressurePreferences []string  `json:"pressure_preferences,omitempty"` // soft, medium, hard
	AvgRating           float64   `db:"avg_rating" json:"avg_rating"`
	TotalReviews        int       `db:"total_reviews" json:"total_reviews"`
	TotalBookings       int       `db:"total_bookings" json:"total_bookings"`
	IsVerified          bool      `db:"is_verified" json:"is_verified"`
	IsAvailable         bool      `db:"is_available" json:"is_available"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

// TherapistDocument represents the therapist_documents table.
type TherapistDocument struct {
	DocumentID   int64      `db:"document_id" json:"document_id"`
	TherapistID  int64      `db:"therapist_id" json:"therapist_id"`
	DocumentType string     `db:"document_type" json:"document_type"`
	DocumentURL  string     `db:"document_url" json:"document_url"`
	Status       string     `db:"status" json:"status"`
	UploadedAt   time.Time  `db:"uploaded_at" json:"uploaded_at"`
	VerifiedAt   *time.Time `db:"verified_at" json:"verified_at,omitempty"`
	VerifiedBy   *int64     `db:"verified_by" json:"verified_by,omitempty"`
}

// TherapistService represents the therapist_services junction table.
type TherapistService struct {
	TherapistServiceID int64     `db:"therapist_service_id" json:"therapist_service_id"`
	TherapistID        int64     `db:"therapist_id" json:"therapist_id"`
	ServiceID          int64     `db:"service_id" json:"service_id"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

// UpdateTherapistProfileRequest for updating profile.
type UpdateTherapistProfileRequest struct {
	Bio             *string `json:"bio"`
	Specialization  *string `json:"specialization"`
	YearsExperience *int    `json:"years_experience"`
	IsAvailable     *bool   `json:"is_available"`
}

// UploadDocumentRequest for document upload.
type UploadDocumentRequest struct {
	DocumentType string `json:"document_type"`
	DocumentURL  string `json:"document_url"`
}

// VerifyDocumentRequest for admin verification.
type VerifyDocumentRequest struct {
	Status string `json:"status"`
}

// AddServiceRequest for linking service to therapist.
type AddServiceRequest struct {
	ServiceID int64 `json:"service_id"`
}

// TherapistProfileResponse to clients.
type TherapistProfileResponse struct {
	TherapistID     int64     `json:"therapist_id"`
	Bio             *string   `json:"bio,omitempty"`
	Specialization  *string   `json:"specialization,omitempty"`
	YearsExperience *int      `json:"years_experience,omitempty"`
	AvgRating       float64   `json:"avg_rating"`
	TotalReviews    int       `json:"total_reviews"`
	TotalBookings   int       `json:"total_bookings"`
	IsVerified      bool      `json:"is_verified"`
	IsAvailable     bool      `json:"is_available"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TherapistDocumentResponse to clients.
type TherapistDocumentResponse struct {
	DocumentID   int64      `json:"document_id"`
	TherapistID  int64      `json:"therapist_id"`
	DocumentType string     `json:"document_type"`
	DocumentURL  string     `json:"document_url"`
	Status       string     `json:"status"`
	UploadedAt   time.Time  `json:"uploaded_at"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	VerifiedBy   *int64     `json:"verified_by,omitempty"`
}
