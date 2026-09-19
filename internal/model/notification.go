package model

import "time"

// Notification represents the notifications table.
type Notification struct {
	NotificationID   int64      `db:"notification_id" json:"notification_id"`
	UserID           int64      `db:"user_id" json:"-"`
	Type             string     `db:"type" json:"type"`
	NotificationType string     `db:"-" json:"notification_type,omitempty"`
	Title            string     `db:"title" json:"title"`
	Message          string     `db:"message" json:"message"`
	Data             []byte     `db:"data" json:"data,omitempty"`
	IsRead           bool       `db:"is_read" json:"is_read"`
	ReadAt           *time.Time `db:"read_at" json:"read_at,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateNotificationRequest for creating notifications.
type CreateNotificationRequest struct {
	UserID           int64          `json:"user_id"`
	Type             string         `json:"type"`
	NotificationType string         `json:"notification_type,omitempty"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	Data             map[string]any `json:"data,omitempty"`
}

// NotificationResponse to clients.
type NotificationResponse struct {
	NotificationID   int64          `json:"notification_id"`
	Type             string         `json:"type"`
	NotificationType string         `json:"notification_type"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	IsRead           bool           `json:"is_read"`
	ReadAt           *time.Time     `json:"read_at,omitempty"`
	Data             map[string]any `json:"data,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// PaginatedNotificationsResponse wraps a list of notifications with pagination metadata.
type PaginatedNotificationsResponse struct {
	Notifications       []NotificationResponse `json:"notifications"`
	Limit               int                    `json:"limit"`
	HasMore             bool                   `json:"has_more"`
	NextCursorCreatedAt *time.Time             `json:"next_cursor_created_at,omitempty"`
	NextCursorID        *int64                 `json:"next_cursor_id,omitempty"`
}

type NotificationUnreadCountResponse struct {
	Unread int `json:"unread"`
}
