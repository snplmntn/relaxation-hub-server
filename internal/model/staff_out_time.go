package model

import "time"

type StaffOutTime struct {
	OutTimeID int64      `db:"out_time_id" json:"out_time_id"`
	UserID    int64      `db:"user_id" json:"user_id"`
	FullName  string     `json:"full_name,omitempty"`
	Role      string     `json:"role,omitempty"`
	WorkDate  time.Time  `db:"work_date" json:"-"`
	Date      string     `json:"work_date"`
	OutAt     time.Time  `db:"out_at" json:"out_at"`
	Notes     string     `db:"notes" json:"notes,omitempty"`
	CreatedBy *int64     `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy *int64     `db:"updated_by" json:"updated_by,omitempty"`
	VoidedBy  *int64     `db:"voided_by" json:"voided_by,omitempty"`
	VoidedAt  *time.Time `db:"voided_at" json:"voided_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

type StaffOutTimeUser struct {
	UserID   int64  `json:"user_id"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type StaffOutTimeFilter struct {
	WorkDate time.Time
	Role     string
	Search   string
}

type StaffOutTimeRequest struct {
	UserID   int64  `json:"user_id"`
	WorkDate string `json:"work_date"`
	OutAt    string `json:"out_at"`
	Notes    string `json:"notes"`
}
