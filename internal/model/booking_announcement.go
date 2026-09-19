package model

import "time"

type BookingAnnouncementVariation struct {
	VariationID    int64     `json:"variation_id"`
	AnnouncementID int64     `json:"announcement_id"`
	Message        string    `json:"message"`
	Position       int       `json:"position"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BookingAnnouncement struct {
	AnnouncementID    int64                          `json:"announcement_id"`
	Message           string                         `json:"-"`
	StartDate         time.Time                      `json:"start_date"`
	EndDate           time.Time                      `json:"end_date"`
	WinnerVariationID *int64                         `json:"winner_variation_id,omitempty"`
	Variations        []BookingAnnouncementVariation `json:"variations"`
	CreatedAt         time.Time                      `json:"created_at"`
	UpdatedAt         time.Time                      `json:"updated_at"`
}

type BookingAnnouncementVariationRequest struct {
	VariationID *int64 `json:"variation_id,omitempty"`
	Message     string `json:"message"`
}

type BookingAnnouncementRequest struct {
	Variations []BookingAnnouncementVariationRequest `json:"variations"`
	StartDate  string                                `json:"start_date"`
	EndDate    string                                `json:"end_date"`
}

type BookingAnnouncementWinnerRequest struct {
	VariationID int64 `json:"variation_id"`
}

type ResolvedBookingAnnouncement struct {
	AnnouncementID int64  `json:"announcement_id"`
	VariationID    int64  `json:"variation_id"`
	Message        string `json:"message"`
}

func ChooseBookingAnnouncementVariation(announcement BookingAnnouncement, subjectID int64) BookingAnnouncementVariation {
	if announcement.WinnerVariationID != nil {
		for _, variation := range announcement.Variations {
			if variation.VariationID == *announcement.WinnerVariationID {
				return variation
			}
		}
	}
	if len(announcement.Variations) == 0 {
		return BookingAnnouncementVariation{}
	}
	index := (subjectID + announcement.AnnouncementID) % int64(len(announcement.Variations))
	return announcement.Variations[index]
}
