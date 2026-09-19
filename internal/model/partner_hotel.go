package model

import "time"

type PartnerHotel struct {
	PartnerHotelID int64     `json:"partner_hotel_id"`
	HotelName      string    `json:"hotel_name"`
	AddressLine    string    `json:"address_line"`
	City           string    `json:"city"`
	ContactPerson  string    `json:"contact_person"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	Notes          string    `json:"notes"`
	IsActive       bool      `json:"is_active"`
	StaffCount     int       `json:"staff_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreatePartnerHotelRequest struct {
	HotelName     string `json:"hotel_name"`
	AddressLine   string `json:"address_line"`
	City          string `json:"city"`
	ContactPerson string `json:"contact_person"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Notes         string `json:"notes"`
}

type UpdatePartnerHotelRequest struct {
	HotelName     *string `json:"hotel_name"`
	AddressLine   *string `json:"address_line"`
	City          *string `json:"city"`
	ContactPerson *string `json:"contact_person"`
	Email         *string `json:"email"`
	Phone         *string `json:"phone"`
	Notes         *string `json:"notes"`
	IsActive      *bool   `json:"is_active"`
}

type PartnerHotelStaff struct {
	UserID              *int64    `json:"user_id"`
	AccessRole          string    `json:"access_role"`
	PasswordHash        string    `json:"-"`
	PartnerHotelStaffID int64     `json:"partner_hotel_staff_id"`
	PartnerHotelID      int64     `json:"partner_hotel_id"`
	FullName            string    `json:"full_name"`
	Position            string    `json:"position"`
	Email               string    `json:"email"`
	Phone               string    `json:"phone"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreatePartnerHotelStaffRequest struct {
	AccessRole string `json:"access_role"`
	Password   string `json:"password"`
	FullName   string `json:"full_name"`
	Position   string `json:"position"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
}

type UpdatePartnerHotelStaffRequest struct {
	AccessRole *string `json:"access_role"`
	Password   *string `json:"password"`
	FullName   *string `json:"full_name"`
	Position   *string `json:"position"`
	Email      *string `json:"email"`
	Phone      *string `json:"phone"`
	IsActive   *bool   `json:"is_active"`
}

const RoleHotelAdmin = "hotel_admin"
const RoleHotelStaff = "hotel_staff"

func IsHotelRole(role string) bool { return role == RoleHotelAdmin || role == RoleHotelStaff }

type HotelAccess struct {
	PartnerHotelID int64  `json:"partner_hotel_id"`
	HotelName      string `json:"hotel_name"`
	City           string `json:"city"`
	StaffID        int64  `json:"staff_id"`
	FullName       string `json:"full_name"`
	AccessRole     string `json:"access_role"`
}
