package model

import "time"

type ApplicationStatus string

const (
	ApplicationStatusPending       ApplicationStatus = "pending"
	ApplicationStatusApproved      ApplicationStatus = "approved"
	ApplicationStatusRejected      ApplicationStatus = "rejected"
	ApplicationStatusNeedsFollow   ApplicationStatus = "needs_followup"
	ApplicationStatusDefault       ApplicationStatus = ApplicationStatusPending
	ApplicationStatusReasonPending                   = "application_pending"
)

type ApplicantApplication struct {
	ApplicationID         int64                  `json:"application_id"`
	UserID                int64                  `json:"user_id"`
	TargetRole            string                 `json:"target_role"`
	PositionApplied       string                 `json:"position_applied"`
	PreferredBranchID     int64                  `json:"preferred_branch_id"`
	PreferredBranchLabel  string                 `json:"preferred_branch_label,omitempty"`
	Status                ApplicationStatus      `json:"status"`
	Answers               map[string]interface{} `json:"answers"`
	Attachments           map[string]interface{} `json:"attachments,omitempty"`
	SubmittedAt           time.Time              `json:"submitted_at"`
	ReviewedAt            *time.Time             `json:"reviewed_at,omitempty"`
	ReviewedByAdminID     *int64                 `json:"reviewed_by_admin_id,omitempty"`
	ReviewNotes           string                 `json:"review_notes,omitempty"`
	ApplicantNameSnapshot string                 `json:"applicant_name,omitempty"`
	ApplicantEmail        string                 `json:"applicant_email,omitempty"`
	ApplicantPhone        string                 `json:"applicant_phone,omitempty"`
}

type CreateApplicationRequest struct {
	Provider             string                 `json:"provider"`
	ProviderKey          string                 `json:"provider_key"`
	Password             string                 `json:"password"`
	TargetRole           string                 `json:"target_role"`
	PositionApplied      string                 `json:"position_applied"`
	PreferredBranchID    int64                  `json:"preferred_branch_id"`
	PreferredBranchLabel string                 `json:"preferred_branch_label,omitempty"`
	FullName             string                 `json:"full_name"`
	PrimaryPhone         string                 `json:"primary_phone,omitempty"`
	Answers              map[string]interface{} `json:"answers,omitempty"`
	Attachments          map[string]interface{} `json:"attachments,omitempty"`
}

type ListApplicationsFilters struct {
	Role   string
	Status string
	Search string
	Page   int
	Limit  int
}

type UpdateApplicationStatusRequest struct {
	Status      ApplicationStatus `json:"status"`
	ReviewNotes string            `json:"review_notes,omitempty"`
}
