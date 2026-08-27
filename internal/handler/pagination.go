package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

const (
	defaultKeysetLimit = 20
	maxKeysetLimit     = 100
)

func parseKeysetPaginationQuery(r *http.Request) (*model.KeysetCursor, int, error) {
	query := r.URL.Query()
	limit := defaultKeysetLimit
	if rawLimit := query.Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	if limit > maxKeysetLimit {
		limit = maxKeysetLimit
	}

	rawCreatedAt := query.Get("cursor_created_at")
	rawID := query.Get("cursor_id")
	if rawCreatedAt == "" && rawID == "" {
		return nil, limit, nil
	}
	if rawCreatedAt == "" || rawID == "" {
		return nil, 0, fmt.Errorf("cursor_created_at and cursor_id must be provided together")
	}

	createdAt, err := time.Parse(time.RFC3339Nano, rawCreatedAt)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid cursor_created_at")
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return nil, 0, fmt.Errorf("invalid cursor_id")
	}

	return &model.KeysetCursor{CreatedAt: createdAt, ID: id}, limit, nil
}
