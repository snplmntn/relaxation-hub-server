package service

import "fmt"

// BlockedAssignmentError indicates a therapist cannot be assigned to a booking
// because a block exists (in either direction) between the therapist and the
// booking's client. Handlers map this to HTTP 409 Conflict.
type BlockedAssignmentError struct {
	TherapistID   int64
	ClientID      int64
	TherapistName string
	ClientName    string
}

func (e *BlockedAssignmentError) Error() string {
	return fmt.Sprintf("%s has blocked %s. Assign a different therapist.", e.ClientName, e.TherapistName)
}

// ValidationError represents a user-facing validation error with a machine
// friendly code and optional field details.
type ValidationError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func (v *ValidationError) Error() string {
	return v.Message
}

// NewValidationError constructs a ValidationError.
func NewValidationError(code, message string, details map[string]string) *ValidationError {
	return &ValidationError{Code: code, Message: message, Details: details}
}
