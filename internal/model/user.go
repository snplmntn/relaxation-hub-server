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

	ProfilePhoto            string                 `json:"profile_photo"`
	Gender                  string                 `json:"gender"`
	EmergencyContactName    string                 `json:"emergency_contact_name"`
	EmergencyContactPhone   string                 `json:"emergency_contact_phone"`
	NotificationPreferences map[string]interface{} `json:"notification_preferences,omitempty"`
	FCMToken                *string                `json:"-"` // FCM token for push notifications (not exposed in API)

	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	Addresses []AddressResponse `json:"addresses,omitempty"`
}

type UserAuthIdentity struct {
	IdentityID   int       `json:"identity_id"`
	UserID       int       `json:"user_id"`
	Provider     string    `json:"provider"`
	ProviderKey  string    `json:"provider_key"`
	PasswordHash string    `json:"-"`
	IsVerified   bool      `json:"is_verified"`
	CreatedAt    time.Time `json:"created_at"`
}

type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// UpdateUserProfileRequest describes updatable user profile fields.
type UpdateUserProfileRequest struct {
	FullName              *string `json:"full_name"`
	Gender                *string `json:"gender"`
	EmergencyContactName  *string `json:"emergency_contact_name"`
	EmergencyContactPhone *string `json:"emergency_contact_phone"`
	ProfilePhoto          *string `json:"profile_photo"`
	PrimaryPhone          *string `json:"primary_phone"`
	Email                 *string `json:"email"`
}

type BlockUserRequest struct {
	BlockedUserID int64 `json:"blocked_user_id"`
}

// UpdateFCMTokenRequest is used to update a user's FCM token for push notifications.
type UpdateFCMTokenRequest struct {
	FCMToken string `json:"fcm_token"`
}
