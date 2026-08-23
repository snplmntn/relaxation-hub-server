package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingCheckoutRepository persists bookings that are mid-payment.
type BookingCheckoutRepository interface {
	Create(ctx context.Context, c *model.BookingCheckout) error
	AttachSession(ctx context.Context, checkoutID int64, sessionID, checkoutURL string) error
	GetByReference(ctx context.Context, reference string) (*model.BookingCheckout, error)
	GetBySessionID(ctx context.Context, sessionID string) (*model.BookingCheckout, error)
	// ClaimForFulfilment marks a pending checkout as being fulfilled by this
	// event, returning false when another delivery already claimed it.
	ClaimForFulfilment(ctx context.Context, checkoutID int64, eventID string) (bool, error)
	// ReleaseClaim hands a claim back after a fulfilment that created nothing,
	// so a retried delivery can win it again.
	ReleaseClaim(ctx context.Context, checkoutID int64) error
	MarkPaid(ctx context.Context, checkoutID int64, bookingID, groupID *int64, note *string) error
	MarkStatus(ctx context.Context, checkoutID int64, status string) error
}

type bookingCheckoutRepo struct {
	db db.DBTX
}

func NewBookingCheckoutRepository(dbtx db.DBTX) BookingCheckoutRepository {
	return &bookingCheckoutRepo{db: dbtx}
}

const bookingCheckoutColumns = `
	checkout_id, reference, client_id, kind, channel, request_payload, amount,
	provider, provider_session_id, checkout_url, status, event_id,
	booking_id, group_id, fulfil_note, expires_at, paid_at, created_at, updated_at`

func scanBookingCheckout(row pgx.Row) (*model.BookingCheckout, error) {
	var c model.BookingCheckout
	if err := row.Scan(
		&c.CheckoutID, &c.Reference, &c.ClientID, &c.Kind, &c.Channel, &c.RequestPayload, &c.Amount,
		&c.Provider, &c.ProviderSessionID, &c.CheckoutURL, &c.Status, &c.EventID,
		&c.BookingID, &c.GroupID, &c.FulfilNote, &c.ExpiresAt, &c.PaidAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *bookingCheckoutRepo) Create(ctx context.Context, c *model.BookingCheckout) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO booking_checkouts (reference, client_id, kind, channel, request_payload, amount, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING checkout_id, provider, status, created_at, updated_at
	`, c.Reference, c.ClientID, c.Kind, c.Channel, string(c.RequestPayload), c.Amount, c.ExpiresAt,
	).Scan(&c.CheckoutID, &c.Provider, &c.Status, &c.CreatedAt, &c.UpdatedAt)
}

func (r *bookingCheckoutRepo) AttachSession(ctx context.Context, checkoutID int64, sessionID, checkoutURL string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE booking_checkouts
		SET provider_session_id = $1, checkout_url = $2, updated_at = CURRENT_TIMESTAMP
		WHERE checkout_id = $3
	`, sessionID, checkoutURL, checkoutID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *bookingCheckoutRepo) GetByReference(ctx context.Context, reference string) (*model.BookingCheckout, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanBookingCheckout(r.db.QueryRow(ctx,
		`SELECT `+bookingCheckoutColumns+` FROM booking_checkouts WHERE reference = $1`, reference))
}

func (r *bookingCheckoutRepo) GetBySessionID(ctx context.Context, sessionID string) (*model.BookingCheckout, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanBookingCheckout(r.db.QueryRow(ctx,
		`SELECT `+bookingCheckoutColumns+` FROM booking_checkouts WHERE provider_session_id = $1`, sessionID))
}

// ClaimForFulfilment is the idempotency gate. PayMongo retries deliveries and
// may send more than one event for a paid session, so the claim is a
// conditional update: only a still-pending row is won, and only once.
func (r *bookingCheckoutRepo) ClaimForFulfilment(ctx context.Context, checkoutID int64, eventID string) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE booking_checkouts
		SET event_id = $1, updated_at = CURRENT_TIMESTAMP
		WHERE checkout_id = $2 AND status = 'pending' AND event_id IS NULL
	`, eventID, checkoutID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() == 1, nil
}

// ReleaseClaim undoes ClaimForFulfilment. It is guarded on 'pending' so a
// fulfilled checkout can never have its event id stripped.
func (r *bookingCheckoutRepo) ReleaseClaim(ctx context.Context, checkoutID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE booking_checkouts
		SET event_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE checkout_id = $1 AND status = 'pending'
	`, checkoutID)
	return err
}

func (r *bookingCheckoutRepo) MarkPaid(ctx context.Context, checkoutID int64, bookingID, groupID *int64, note *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE booking_checkouts
		SET status = 'paid', booking_id = $1, group_id = $2, fulfil_note = $3,
		    paid_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE checkout_id = $4
	`, bookingID, groupID, note, checkoutID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *bookingCheckoutRepo) MarkStatus(ctx context.Context, checkoutID int64, status string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE booking_checkouts SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE checkout_id = $2
	`, status, checkoutID)
	return err
}
