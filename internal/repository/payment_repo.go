package repository

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// PaymentRepository handles payments persistence.
type PaymentRepository interface {
	Create(ctx context.Context, p *model.Payment) error
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error)
	GetByBookingIDBatch(ctx context.Context, bookingIDs []int64) (map[int64]*model.Payment, error)
	GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error)
	UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error
	UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error
	Verify(ctx context.Context, bookingID int64, verifiedBy int64, notes *string) error
	Reject(ctx context.Context, bookingID int64, rejectedBy int64, notes *string) error
	ClearProof(ctx context.Context, bookingID int64) error
}

type paymentRepoImpl struct {
	db db.DBTX
}

func NewPaymentRepository(db db.DBTX) PaymentRepository {
	return &paymentRepoImpl{db: db}
}

func (r *paymentRepoImpl) Create(ctx context.Context, p *model.Payment) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
        INSERT INTO payments (
            booking_id, amount, gateway, transaction_id, status, webhook_id
        ) VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING payment_id, transaction_date, paid_at, refunded_at, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		p.BookingID,
		p.Amount,
		p.Gateway,
		p.TransactionID,
		p.Status,
		p.WebhookID,
	).Scan(&p.PaymentID, &p.TransactionAt, &p.PaidAt, &p.RefundedAt, &p.CreatedAt, &p.UpdatedAt)
}

func (r *paymentRepoImpl) GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
        SELECT payment_id, booking_id, amount, gateway, transaction_id, status, gateway_response,
               webhook_id, proof_url, verified_at, verified_by, notes, transaction_date, paid_at, refunded_at, created_at, updated_at
        FROM payments WHERE booking_id = $1
    `
	var p model.Payment
	if err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&p.PaymentID,
		&p.BookingID,
		&p.Amount,
		&p.Gateway,
		&p.TransactionID,
		&p.Status,
		&p.GatewayPayload,
		&p.WebhookID,
		&p.ProofURL,
		&p.VerifiedAt,
		&p.VerifiedBy,
		&p.Notes,
		&p.TransactionAt,
		&p.PaidAt,
		&p.RefundedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}
	return &p, nil
}

// GetByBookingIDBatch fetches payments for multiple bookings in a single query.
func (r *paymentRepoImpl) GetByBookingIDBatch(ctx context.Context, bookingIDs []int64) (map[int64]*model.Payment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(bookingIDs) == 0 {
		return make(map[int64]*model.Payment), nil
	}

	query := `
        SELECT payment_id, booking_id, amount, gateway, transaction_id, status, gateway_response,
               webhook_id, proof_url, verified_at, verified_by, notes, transaction_date, paid_at, refunded_at, created_at, updated_at
        FROM payments WHERE booking_id = ANY($1)
    `
	rows, err := r.db.Query(ctx, query, bookingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]*model.Payment)
	for rows.Next() {
		var p model.Payment
		if err := rows.Scan(
			&p.PaymentID,
			&p.BookingID,
			&p.Amount,
			&p.Gateway,
			&p.TransactionID,
			&p.Status,
			&p.GatewayPayload,
			&p.WebhookID,
			&p.ProofURL,
			&p.VerifiedAt,
			&p.VerifiedBy,
			&p.Notes,
			&p.TransactionAt,
			&p.PaidAt,
			&p.RefundedAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result[p.BookingID] = &p
	}

	return result, rows.Err()
}

// GetOrCreateByBookingID retrieves an existing payment or creates a new pending one.
func (r *paymentRepoImpl) GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error) {
	// Try to get existing
	p, err := r.GetByBookingID(ctx, bookingID)
	if err == nil {
		return p, nil
	}
	if err != pgx.ErrNoRows {
		slog.Warn("[PaymentRepo] GetByBookingID failed", "booking_id", bookingID, "error", err)
		return nil, err
	}

	slog.Info("[PaymentRepo] No payment found, creating new pending payment", "booking_id", bookingID)

	// Create new pending payment
	newPayment := &model.Payment{
		BookingID: bookingID,
		Amount:    amount,
		Gateway:   gateway,
		Status:    "pending",
	}
	if err := r.Create(ctx, newPayment); err != nil {
		slog.Error("[PaymentRepo] Create payment failed", "booking_id", bookingID, "error", err)
		return nil, err
	}
	slog.Info("[PaymentRepo] Created payment", "payment_id", newPayment.PaymentID, "booking_id", bookingID)
	return newPayment, nil
}

func (r *paymentRepoImpl) UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET status = $1,
            transaction_id = COALESCE($2, transaction_id),
            webhook_id = COALESCE($3, webhook_id),
            paid_at = CASE WHEN $1 = 'paid' THEN CURRENT_TIMESTAMP ELSE paid_at END,
            refunded_at = CASE WHEN $1 = 'refunded' THEN CURRENT_TIMESTAMP ELSE refunded_at END,
            updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $4
    `, status, transactionID, webhookID, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateProofURL updates the proof_url for a payment.
func (r *paymentRepoImpl) UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET proof_url = $1, updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $2
    `, proofURL, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Verify marks a payment as paid and verified.
func (r *paymentRepoImpl) Verify(ctx context.Context, bookingID int64, verifiedBy int64, notes *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET status = 'paid', verified_at = CURRENT_TIMESTAMP, verified_by = $1, paid_at = CURRENT_TIMESTAMP, notes = COALESCE($2, notes), updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $3
    `, verifiedBy, notes, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Reject marks a payment as rejected.
func (r *paymentRepoImpl) Reject(ctx context.Context, bookingID int64, rejectedBy int64, notes *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET status = 'rejected', verified_at = CURRENT_TIMESTAMP, verified_by = $1, notes = $2, updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $3
    `, rejectedBy, notes, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ClearProof removes the proof URL and resets the status to pending.
func (r *paymentRepoImpl) ClearProof(ctx context.Context, bookingID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET proof_url = NULL, status = 'pending', updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $1
    `, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
