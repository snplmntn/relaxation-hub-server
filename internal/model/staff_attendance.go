package model

import "time"

type StaffAttendance struct {
	AttendanceID int64      `db:"attendance_id" json:"attendance_id"`
	UserID       int64      `db:"user_id" json:"user_id"`
	FullName     string     `db:"full_name" json:"full_name,omitempty"`
	Role         string     `db:"role" json:"role,omitempty"`
	WorkDate     time.Time  `db:"work_date" json:"-"`
	Date         string     `json:"work_date"`
	TimeInAt     *time.Time `db:"time_in_at" json:"time_in_at,omitempty"`
	TimeOutAt    *time.Time `db:"time_out_at" json:"time_out_at,omitempty"`
	Notes        string     `db:"notes" json:"notes,omitempty"`
	CreatedBy    *int64     `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy    *int64     `db:"updated_by" json:"updated_by,omitempty"`
	VoidedBy     *int64     `db:"voided_by" json:"voided_by,omitempty"`
	VoidedAt     *time.Time `db:"voided_at" json:"voided_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

type StaffAttendanceUser struct {
	UserID   int64  `json:"user_id"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type StaffAttendanceFilter struct {
	WorkDate time.Time
	Role     string
	Search   string
}

type StaffAttendanceRequest struct {
	UserID    int64  `json:"user_id"`
	WorkDate  string `json:"work_date"`
	TimeInAt  string `json:"time_in_at"`
	TimeOutAt string `json:"time_out_at"`
	Notes     string `json:"notes"`
}
