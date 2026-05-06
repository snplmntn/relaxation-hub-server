package model

import "time"

type DayViewTherapistOrder struct {
	OrderID           int64
	ViewKey           string
	BusinessDate      time.Time
	TherapistIDs      []int64
	Source            string
	UpdatedByAdminID  *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DayViewTherapistCandidate struct {
	TherapistID int64
	Name        string
}
