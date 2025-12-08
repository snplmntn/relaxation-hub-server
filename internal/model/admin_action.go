package model

import "time"

// AdminAction represents the admin_actions table for audit logging.
type AdminAction struct {
	ActionID     int64     `db:"action_id" json:"action_id"`
	AdminID      int64     `db:"admin_id" json:"admin_id"`
	ActionType   string    `db:"action_type" json:"action_type"`
	TargetType   *string   `db:"target_type" json:"target_type,omitempty"`
	TargetID     *int64    `db:"target_id" json:"target_id,omitempty"`
	Description  *string   `db:"description" json:"description,omitempty"`
	OldValue     *string   `db:"old_value" json:"old_value,omitempty"`
	NewValue     *string   `db:"new_value" json:"new_value,omitempty"`
	IPAddress    *string   `db:"ip_address" json:"ip_address,omitempty"`
	PerformedAt  time.Time `db:"performed_at" json:"performed_at"`
}

// CreateAdminActionRequest for logging admin action.
type CreateAdminActionRequest struct {
	ActionType  string  `json:"action_type"`
	TargetType  *string `json:"target_type"`
	TargetID    *int64  `json:"target_id"`
	Description *string `json:"description"`
	OldValue    *string `json:"old_value"`
	NewValue    *string `json:"new_value"`
	IPAddress   *string `json:"ip_address"`
}

// AdminActionResponse to clients.
type AdminActionResponse struct {
	ActionID    int64     `json:"action_id"`
	AdminID     int64     `json:"admin_id"`
	ActionType  string    `json:"action_type"`
	TargetType  *string   `json:"target_type,omitempty"`
	TargetID    *int64    `json:"target_id,omitempty"`
	Description *string   `json:"description,omitempty"`
	OldValue    *string   `json:"old_value,omitempty"`
	NewValue    *string   `json:"new_value,omitempty"`
	IPAddress   *string   `json:"ip_address,omitempty"`
	PerformedAt time.Time `json:"performed_at"`
}
