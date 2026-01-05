package repository

import (
	"context"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ExtensionRequestRepository handles database operations for extension requests
type ExtensionRequestRepository interface {
	Create(ctx context.Context, req *model.ExtensionRequest) error
	GetByID(ctx context.Context, requestID int64) (*model.ExtensionRequest, error)
	GetPendingByBookingID(ctx context.Context, bookingID int64) (*model.ExtensionRequest, error)
	UpdateStatus(ctx context.Context, requestID int64, status string, respondedBy int64, note *string) error
	ListByBookingID(ctx context.Context, bookingID int64) ([]model.ExtensionRequest, error)
}

type extensionRequestRepo struct {
	db db.DBTX
}

// NewExtensionRequestRepository creates a new repository
func NewExtensionRequestRepository(db db.DBTX) ExtensionRequestRepository {
	return &extensionRequestRepo{db: db}
}

func (r *extensionRequestRepo) Create(ctx context.Context, req *model.ExtensionRequest) error {
	query := `
		INSERT INTO booking_extension_requests 
			(booking_id, requested_minutes, additional_cost, status, requested_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING request_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		req.BookingID,
		req.RequestedMinutes,
		req.AdditionalCost,
		model.ExtensionStatusPending,
		req.RequestedBy,
	).Scan(&req.RequestID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *extensionRequestRepo) GetByID(ctx context.Context, requestID int64) (*model.ExtensionRequest, error) {
	query := `
		SELECT request_id, booking_id, requested_minutes, additional_cost, status, 
		       requested_by, responded_by, response_note, created_at, updated_at
		FROM booking_extension_requests
		WHERE request_id = $1
	`
	req := &model.ExtensionRequest{}
	err := r.db.QueryRow(ctx, query, requestID).Scan(
		&req.RequestID, &req.BookingID, &req.RequestedMinutes, &req.AdditionalCost,
		&req.Status, &req.RequestedBy, &req.RespondedBy, &req.ResponseNote,
		&req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (r *extensionRequestRepo) GetPendingByBookingID(ctx context.Context, bookingID int64) (*model.ExtensionRequest, error) {
	query := `
		SELECT request_id, booking_id, requested_minutes, additional_cost, status, 
		       requested_by, responded_by, response_note, created_at, updated_at
		FROM booking_extension_requests
		WHERE booking_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`
	req := &model.ExtensionRequest{}
	err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&req.RequestID, &req.BookingID, &req.RequestedMinutes, &req.AdditionalCost,
		&req.Status, &req.RequestedBy, &req.RespondedBy, &req.ResponseNote,
		&req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (r *extensionRequestRepo) UpdateStatus(ctx context.Context, requestID int64, status string, respondedBy int64, note *string) error {
	query := `
		UPDATE booking_extension_requests
		SET status = $1, responded_by = $2, response_note = $3, updated_at = $4
		WHERE request_id = $5
	`
	_, err := r.db.Exec(ctx, query, status, respondedBy, note, time.Now(), requestID)
	return err
}

func (r *extensionRequestRepo) ListByBookingID(ctx context.Context, bookingID int64) ([]model.ExtensionRequest, error) {
	query := `
		SELECT request_id, booking_id, requested_minutes, additional_cost, status, 
		       requested_by, responded_by, response_note, created_at, updated_at
		FROM booking_extension_requests
		WHERE booking_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.ExtensionRequest
	for rows.Next() {
		var req model.ExtensionRequest
		if err := rows.Scan(
			&req.RequestID, &req.BookingID, &req.RequestedMinutes, &req.AdditionalCost,
			&req.Status, &req.RequestedBy, &req.RespondedBy, &req.ResponseNote,
			&req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, req)
	}
	return results, rows.Err()
}
