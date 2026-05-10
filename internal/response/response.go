package response

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a standardized error response.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
	// Retryable indicates whether the client may retry the request.
	Retryable bool `json:"retryable,omitempty"`
	// RetryAfterSeconds suggests a wait time (in seconds) before retrying.
	RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`
}

// SuccessResponse represents a standardized success response.
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RespondError writes an error response
func RespondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	er := ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	}

	// Provide guidance for retryable server errors and rate limits.
	switch status {
	case http.StatusTooManyRequests:
		er.Retryable = true
		er.RetryAfterSeconds = 30
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusInternalServerError:
		er.Retryable = true
		// default conservative hint
		er.RetryAfterSeconds = 60
	default:
		// non-retryable by default for 4xx client errors
	}

	_ = json.NewEncoder(w).Encode(er)
}

// RespondValidation writes a 4xx validation error with code and details.
func RespondValidation(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(status),
		Code:    code,
		Message: message,
		Details: details,
	})
}

// RespondSuccess writes a success response
func RespondSuccess(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{
		Message: message,
		Data:    data,
	})
}

// RespondJSON writes a raw JSON response (no envelope).
func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}
