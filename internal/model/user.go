package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	UserID       int    `json:"user_id"`
	FullName     string `json:"full_name"`
	Role         string `json:"role"`
	PrimaryEmail string `json:"primary_email"`
	PrimaryPhone string `json:"primary_phone"`

	ProfilePhoto          string `json:"profile_photo"`
	Gender                string `json:"gender"`
	EmergencyContactName  string `json:"emergency_contact_name"`
	EmergencyContactPhone string `json:"emergency_contact_phone"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserAuthIdentity struct {
	IdentityID   int    `json:"identity_id"`
	UserID       int    `json:"user_id"`
	Provider     string `json:"provider"`
	ProviderKey  string `json:"provider_key"`
	Password string `json:"-"`
	IsVerified   bool   `json:"is_verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}