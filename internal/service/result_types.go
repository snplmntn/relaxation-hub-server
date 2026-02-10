package service

import (
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingWithTimelineResult contains a booking with all related data for timeline viewing.
// This struct replaces the 15-value return signature of GetBookingWithTimeline/GetBookingByCodeWithTimeline.
type BookingWithTimelineResult struct {
	Booking         *model.Booking        `json:"booking"`
	Events          []model.BookingEvent  `json:"events"`
	Service         *model.Service        `json:"service,omitempty"`
	Address         *model.Address        `json:"address,omitempty"`
	ActiveRide      *model.Ride           `json:"active_ride,omitempty"`
	
	// Therapist info
	TherapistName   string                `json:"therapist_name,omitempty"`
	TherapistPhone  string                `json:"therapist_phone,omitempty"`
	TherapistPhoto  string                `json:"therapist_photo,omitempty"`
	TherapistGender string                `json:"therapist_gender,omitempty"`
	TherapistRating *float64              `json:"therapist_rating,omitempty"`
	
	// Client info
	ClientName      string                `json:"client_name,omitempty"`
	ClientPhone     string                `json:"client_phone,omitempty"`
	ClientPhoto     string                `json:"client_photo,omitempty"`
	ClientGender    string                `json:"client_gender,omitempty"`
	
	// Promo
	PromoCode       string                `json:"promo_code,omitempty"`
}

// UserInfo represents basic user information (used in result structs).
type UserInfo struct {
	Name   string  `json:"name"`
	Phone  string  `json:"phone"`
	Photo  string  `json:"photo"`
	Gender string  `json:"gender"`
	Rating *float64 `json:"rating,omitempty"`
}

// BookingCreateResult contains the result of a booking creation operation.
type BookingCreateResult struct {
	Booking   *model.Booking `json:"booking"`
	Offer     *model.BookingOffer `json:"offer,omitempty"`
	Therapist *UserInfo `json:"therapist,omitempty"`
}
