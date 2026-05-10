package service

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
