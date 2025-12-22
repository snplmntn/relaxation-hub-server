package handler

import (
	resp "github.com/snplmntn/relaxation-hub-server/internal/response"
	"net/http"
)

// respondError delegates to the shared response package.
func respondError(w http.ResponseWriter, status int, message string) {
	resp.RespondError(w, status, message)
}

// respondValidation delegates to the shared response package.
func respondValidation(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	resp.RespondValidation(w, status, code, message, details)
}

// respondSuccess delegates to the shared response package.
func respondSuccess(w http.ResponseWriter, message string, data interface{}) {
	resp.RespondSuccess(w, message, data)
}

// Re-export types for tests and existing callers in the handler package.
type ErrorResponse = resp.ErrorResponse
type SuccessResponse = resp.SuccessResponse
