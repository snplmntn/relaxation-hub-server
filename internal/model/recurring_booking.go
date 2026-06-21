package model

import "time"

// RecurringBooking represents a template that generates individual bookings on a schedule.
type RecurringBooking struct {
	RecurringID     int64      `db:"recurring_id" json:"recurring_id"`
	ClientID        int64      `db:"client_id" json:"client_id"`
	CreatedBy       *int64     `db:"created_by" json:"created_by,omitempty"`
	ServiceID       *int64     `db:"service_id" json:"service_id,omitempty"`
	AddressID       *int64     `db:"address_id" json:"address_id,omitempty"`
	TherapistID     *int64     `db:"therapist_id" json:"therapist_id,omitempty"`
	DurationMinutes int        `db:"duration_minutes" json:"duration_minutes"`
	GenderPref      string     `db:"gender_preference" json:"gender_preference"`
	PressurePref    string     `db:"pressure_preference" json:"pressure_preference"`
	Notes           string     `db:"notes" json:"notes"`
	PaymentMethod   string     `db:"payment_method" json:"payment_method"`
	Frequency       string     `db:"frequency" json:"frequency"`         // daily | weekly | monthly
	IntervalValue   int        `db:"interval_value" json:"interval_value"` // repeat every N freq units
	DaysOfWeek      []int      `db:"days_of_week" json:"days_of_week"`   // 0=Sun…6=Sat (weekly only)
	DayOfMonth      *int       `db:"day_of_month" json:"day_of_month,omitempty"` // 1–31 (monthly only)
	TimeOfDay       string     `db:"time_of_day" json:"time_of_day"`     // HH:MM (24h)
	StartDate       time.Time  `db:"start_date" json:"start_date"`
	EndDate         *time.Time `db:"end_date" json:"end_date,omitempty"`
	Status          string     `db:"status" json:"status"` // active | paused | cancelled
	GeneratedUntil  *time.Time `db:"generated_until" json:"generated_until,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`

	// Hydrated fields
	UpcomingBookings []Booking `db:"-" json:"upcoming_bookings,omitempty"`
}

// CreateRecurringBookingRequest is the admin payload for starting a series.
type CreateRecurringBookingRequest struct {
	ClientID        int64    `json:"client_id"`
	ServiceID       *int64   `json:"service_id"`
	AddressID       *int64   `json:"address_id"`
	TherapistID     *int64   `json:"therapist_id"`
	DurationMinutes int      `json:"duration_minutes"`
	GenderPref      string   `json:"gender_preference"`
	PressurePref    string   `json:"pressure_preference"`
	Notes           string   `json:"notes"`
	PaymentMethod   string   `json:"payment_method"`
	Frequency       string   `json:"frequency"`
	IntervalValue   int      `json:"interval_value"`
	DaysOfWeek      []int    `json:"days_of_week"`
	DayOfMonth      *int     `json:"day_of_month,omitempty"`
	TimeOfDay       string   `json:"time_of_day"`   // HH:MM
	StartDate       string   `json:"start_date"`    // YYYY-MM-DD
	EndDate         string   `json:"end_date,omitempty"` // YYYY-MM-DD or ""
}

// UpdateRecurringBookingRequest allows status changes and schedule edits.
type UpdateRecurringBookingRequest struct {
	Status        *string `json:"status,omitempty"` // active | paused | cancelled
	EndDate       *string `json:"end_date,omitempty"`
	Notes         *string `json:"notes,omitempty"`
	PaymentMethod *string `json:"payment_method,omitempty"`
	TherapistID   *int64  `json:"therapist_id,omitempty"`
}
