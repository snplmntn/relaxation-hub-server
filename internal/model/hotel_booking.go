package model

import "time"

type HotelBooking struct {
	BookingID       int64      `json:"booking_id"`
	ClientID        int64      `json:"-"`
	HotelName       string     `json:"hotel_name"`
	GuestName       string     `json:"guest_name"`
	Notes           string     `json:"notes"`
	Status          string     `json:"status"`
	ScheduledStart  *time.Time `json:"scheduled_start"`
	DurationMinutes int        `json:"duration_minutes"`
	ServiceName     string     `json:"service_name"`
	TherapistName   string     `json:"therapist_name"`
}

// HotelBookingOption is the minimum hotel-account data operational staff need
// when creating a booking on a hotel's behalf.
type HotelBookingOption struct {
	PartnerHotelID   int64  `json:"partner_hotel_id"`
	HotelName        string `json:"hotel_name"`
	AddressLine      string `json:"address_line"`
	City             string `json:"city"`
	BookingClientID  *int64 `json:"booking_client_id,omitempty"`
	DefaultAddressID *int64 `json:"default_address_id,omitempty"`
}

// HotelAnalyticsOption is the non-sensitive hotel directory exposed to
// operational staff for selecting a performance report.
type HotelAnalyticsOption struct {
	PartnerHotelID int64  `json:"partner_hotel_id"`
	HotelName      string `json:"hotel_name"`
	City           string `json:"city"`
	IsActive       bool   `json:"is_active"`
}

type HotelAnalytics struct {
	HotelName      string                        `json:"hotel_name"`
	Currency       string                        `json:"currency"`
	Period         HotelAnalyticsPeriod          `json:"period"`
	Summary        HotelAnalyticsSummary         `json:"summary"`
	Daily          []HotelAnalyticsDaily         `json:"daily"`
	Statuses       []HotelAnalyticsStatus        `json:"statuses"`
	TopServices    []HotelAnalyticsService       `json:"top_services"`
	RecentBookings []HotelAnalyticsRecentBooking `json:"recent_bookings"`
}

type HotelAnalyticsPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type HotelAnalyticsSummary struct {
	TotalBookings       int     `json:"total_bookings"`
	CompletedBookings   int     `json:"completed_bookings"`
	UpcomingBookings    int     `json:"upcoming_bookings"`
	CancelledBookings   int     `json:"cancelled_bookings"`
	Revenue             float64 `json:"revenue"`
	BookedValue         float64 `json:"booked_value"`
	AverageBookingValue float64 `json:"average_booking_value"`
	CompletionRate      float64 `json:"completion_rate"`
	CancellationRate    float64 `json:"cancellation_rate"`
}

type HotelAnalyticsDaily struct {
	Date     string  `json:"date"`
	Bookings int     `json:"bookings"`
	Revenue  float64 `json:"revenue"`
}

type HotelAnalyticsStatus struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type HotelAnalyticsService struct {
	ServiceName string  `json:"service_name"`
	Bookings    int     `json:"bookings"`
	Revenue     float64 `json:"revenue"`
}

type HotelAnalyticsRecentBooking struct {
	BookingID      int64      `json:"booking_id"`
	GuestName      string     `json:"guest_name"`
	ServiceName    string     `json:"service_name"`
	Status         string     `json:"status"`
	ScheduledStart *time.Time `json:"scheduled_start"`
	FinalTotal     float64    `json:"final_total"`
}

type UpdateHotelBookingRequest struct {
	GuestName string `json:"guest_name"`
	Notes     string `json:"notes"`
}
