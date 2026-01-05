package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// PaymentRepository handles payments persistence.
type PaymentRepository interface {
	Create(ctx context.Context, p *model.Payment) error
	GetByBookingID(ctx context.Context, bookingID int64) (*model.Payment, error)
	GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error)
	UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error
	UpdateProofURL(ctx context.Context, bookingID int64, proofURL string) error
	Verify(ctx context.Context, bookingID int64, verifiedBy int64) error
}

type paymentRepoImpl struct {
	db db.DBTX
}

func NewPaymentRepository(db db.DBTX) PaymentRepository {
	return &paymentRepoImpl{db: db}
}

func (r *paymentRepoImpl) Create(ctx context.Context, p *model.Payment) error {
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
	query := `
        SELECT payment_id, booking_id, amount, gateway, transaction_id, status, gateway_response,
               webhook_id, proof_url, verified_at, verified_by, transaction_date, paid_at, refunded_at, created_at, updated_at
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

// GetOrCreateByBookingID retrieves an existing payment or creates a new pending one.
func (r *paymentRepoImpl) GetOrCreateByBookingID(ctx context.Context, bookingID int64, amount float64, gateway string) (*model.Payment, error) {
	// Try to get existing
	p, err := r.GetByBookingID(ctx, bookingID)
	if err == nil {
		return p, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	// Create new pending payment
	newPayment := &model.Payment{
		BookingID: bookingID,
		Amount:    amount,
		Gateway:   gateway,
		Status:    "pending",
	}
	if err := r.Create(ctx, newPayment); err != nil {
		return nil, err
	}
	return newPayment, nil
}

func (r *paymentRepoImpl) UpdateStatus(ctx context.Context, bookingID int64, status string, transactionID *string, webhookID *string) error {
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

// Verify marks a payment as verified.
func (r *paymentRepoImpl) Verify(ctx context.Context, bookingID int64, verifiedBy int64) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE payments
        SET status = 'verified', verified_at = CURRENT_TIMESTAMP, verified_by = $1, updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $2
    `, verifiedBy, bookingID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

