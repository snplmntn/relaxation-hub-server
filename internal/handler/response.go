package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	resp "github.com/snplmntn/relaxation-hub-server/internal/response"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// respondError delegates to the shared response package.
func respondError(w http.ResponseWriter, status int, message string) {
	resp.RespondError(w, status, message)
}

func respondServiceError(w http.ResponseWriter, status int, err error) {
	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		respondValidation(w, status, validationErr.Code, validationErr.Message, validationErr.Details)
		return
	}

	if status >= http.StatusBadRequest && status < http.StatusInternalServerError && !isInternalServiceError(err) {
		resp.RespondError(w, status, err.Error())
		return
	}
	resp.RespondError(w, http.StatusInternalServerError, err.Error())
}

// respondClientError is only for errors explicitly matched to a domain sentinel by the caller.
func respondClientError(w http.ResponseWriter, status int, err error) {
	resp.RespondError(w, status, err.Error())
}

func isInternalServiceError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Unwrap(err) != nil {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{
		"failed to ", "could not ", "database ", "db error", "sqlstate", "connection ",
		"context deadline", "not configured", "not initialized", "querying ", "scanning ",
		"smtp ", "s3 ", "geocoding ", "mapbox ", "osrm ", "fcm ", "serialize ", "decode ",
		" not found:", " failed ", "unable to ", "unexpected ",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return strings.HasSuffix(message, " failed")
}

// respondValidation delegates to the shared response package.
func respondValidation(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	resp.RespondValidation(w, status, code, message, details)
}

// respondSuccess delegates to the shared response package.
func respondSuccess(w http.ResponseWriter, message string, data interface{}) {
	resp.RespondSuccess(w, message, data)
}

// respondJSON delegates to the shared response package for raw JSON output.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	resp.RespondJSON(w, status, payload)
}

// Re-export types for tests and existing callers in the handler package.
type ErrorResponse = resp.ErrorResponse
type SuccessResponse = resp.SuccessResponse
