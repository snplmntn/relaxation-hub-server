package model

import "time"

// Identifying fields are populated only for bookings in the viewer's hotel.
type HotelBookedSlot struct {
	BookingID int64     `json:"booking_id,omitempty"`
	HotelName string    `json:"hotel_name,omitempty"`
	GuestName string    `json:"guest_name,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
}

type HotelDayViewTherapist struct {
	TherapistID       int64             `json:"therapist_id"`
	Name              string            `json:"name"`
	BranchID          *int64            `json:"branch_id"`
	AcceptingBookings bool              `json:"accepting_bookings"`
	BookedSlots       []HotelBookedSlot `json:"booked_slots"`
}

type HotelDayViewBranch struct {
	BranchID int64  `json:"branch_id"`
	Name     string `json:"name"`
}

type HotelDayView struct {
	Date       string                  `json:"date"`
	Start      time.Time               `json:"start"`
	End        time.Time               `json:"end"`
	Timezone   string                  `json:"timezone"`
	Branches   []HotelDayViewBranch    `json:"branches"`
	Therapists []HotelDayViewTherapist `json:"therapists"`
}
