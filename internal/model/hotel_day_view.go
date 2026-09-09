package model

import "time"

// Hotel day view deliberately has no booking DTOs or booking identifiers.
type HotelBookedSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
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
