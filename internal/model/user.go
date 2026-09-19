package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	UserID        int    `json:"user_id"`
	FullName      string `json:"full_name"`
	Role          string `json:"role"`
	PrimaryEmail  string `json:"primary_email"`
	PrimaryPhone  string `json:"primary_phone"`
	AccountStatus string `json:"account_status"`
	StatusReason  string `json:"status_reason,omitempty"`
	IsVIP         bool   `json:"is_vip"`

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
	Rider     *RiderProfile     `json:"rider,omitempty"`
}

// UserStatusCounts holds aggregate counts for a role's roster, broken down by
// account_status (plus VIP). Used by the admin roster summary cards so the
// totals reflect the whole dataset rather than a single loaded page.
type UserStatusCounts struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Inactive  int `json:"inactive"`
	Suspended int `json:"suspended"`
	Blocked   int `json:"blocked"`
	Banned    int `json:"banned"`
	VIP       int `json:"vip"`
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
	FullName                *string                `json:"full_name"`
	Gender                  *string                `json:"gender"`
	EmergencyContactName    *string                `json:"emergency_contact_name"`
	EmergencyContactPhone   *string                `json:"emergency_contact_phone"`
	ProfilePhoto            *string                `json:"profile_photo"`
	PrimaryPhone            *string                `json:"primary_phone"`
	Email                   *string                `json:"email"`
	IsVIP                   *bool                  `json:"is_vip"`
	NotificationPreferences map[string]interface{} `json:"notification_preferences"`
}

type BlockUserRequest struct {
	BlockedUserID int64 `json:"blocked_user_id"`
}

type UnblockUserRequest struct {
	UnblockedUserID int64 `json:"unblocked_user_id"`
}

// UpdateFCMTokenRequest is used to update a user's FCM token for push notifications.
type UpdateFCMTokenRequest struct {
	FCMToken string `json:"fcm_token"`
}
