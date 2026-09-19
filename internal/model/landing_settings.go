package model

import "time"

// LandingSettings holds the public landing page contact / social / application
// info managed from the Super Admin dashboard. It is persisted as a single row.
type LandingSettings struct {
	PhoneGlobe      string    `json:"phone_globe"`
	PhoneSmart      string    `json:"phone_smart"`
	Email           string    `json:"email"`
	Address         string    `json:"address"`
	Hours           string    `json:"hours"`
	FacebookURL     string    `json:"facebook_url"`
	InstagramURL    string    `json:"instagram_url"`
	WhatsappURL     string    `json:"whatsapp_url"`
	ViberURL        string    `json:"viber_url"`
	ApplicationLink string    `json:"application_link"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// LandingSettingsColumns maps JSON field names to their database columns. Only
// these keys are accepted on update, guarding the dynamic UPDATE against
// arbitrary column injection.
var LandingSettingsColumns = map[string]string{
	"phone_globe":      "phone_globe",
	"phone_smart":      "phone_smart",
	"email":            "email",
	"address":          "address",
	"hours":            "hours",
	"facebook_url":     "facebook_url",
	"instagram_url":    "instagram_url",
	"whatsapp_url":     "whatsapp_url",
	"viber_url":        "viber_url",
	"application_link": "application_link",
}
