package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// PromotionRepository manages promotions.
type PromotionRepository interface {
	Create(ctx context.Context, p *model.Promotion) error
	// ListActive returns in-date promotions. publicOnly restricts the result to
	// codes clients may see; staff listings pass false.
	ListActive(ctx context.Context, now time.Time, publicOnly bool) ([]model.Promotion, error)
	GetByCode(ctx context.Context, code string) (*model.Promotion, error)
	// GetByID loads a promo by id, for repricing a booking that already has one attached.
	GetByID(ctx context.Context, promoID int64) (*model.Promotion, error)
	// TryIncrementGlobalUsageTx increments `current_uses` for a promo inside
	// the provided transaction if the promo has remaining uses. Returns true
	// if the increment succeeded, false if the promo is exhausted.
	TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error)
	// TryIncrementUserPromoUsageTx increments (or creates) a row in
	// `user_promotions` for the given user/promo inside the provided
	// transaction. Returns true on success.
	TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error)
	// ListAll returns all regular promotions (excluding deleted).
	ListAll(ctx context.Context) ([]model.Promotion, error)
	// ListBookings returns every booking that carries the promotion, including
	// cancelled rows for audit visibility.
	ListBookings(ctx context.Context, promoID int64) ([]model.VoucherBooking, error)
	// ListAllVoucherBookings returns bookings across every promotion so admins
	// can filter the full voucher ledger without issuing one request per code.
	ListAllVoucherBookings(ctx context.Context) ([]model.VoucherBooking, error)
	// Update updates a promotion.
	Update(ctx context.Context, promoID int64, updates map[string]interface{}) error
	// Delete performs a soft delete.
	Delete(ctx context.Context, promoID int64) error
}

type promotionRepoImpl struct {
	db db.DBTX
}

func NewPromotionRepository(db db.DBTX) PromotionRepository {
	return &promotionRepoImpl{db: db}
}

func (r *promotionRepoImpl) Create(ctx context.Context, p *model.Promotion) error {
	query := `
        INSERT INTO promotions (
            code, discount_percentage, discount_amount, applies_to, valid_from, valid_until, max_uses,
            days_of_week, start_time, end_time, is_public
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        RETURNING promo_id, current_uses, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		p.Code,
		p.DiscountPct,
		p.DiscountAmount,
		p.AppliesTo,
		p.ValidFrom,
		p.ValidUntil,
		p.UsageLimit,
		p.DaysOfWeek,
		p.StartTime,
		p.EndTime,
		p.IsPublic,
	).Scan(&p.PromoID, &p.CurrentUses, &p.CreatedAt, &p.UpdatedAt)
}

func (r *promotionRepoImpl) ListActive(ctx context.Context, now time.Time, publicOnly bool) ([]model.Promotion, error) {
	visibility := ""
	if publicOnly {
		visibility = " AND p.is_public"
	}
	query := `
		SELECT p.promo_id, p.code, p.discount_percentage, p.discount_amount, p.applies_to,
		       p.valid_from, p.valid_until, p.max_uses,
		       COALESCE((
		           SELECT COUNT(DISTINCT COALESCE('g' || b.group_id, 'b' || b.booking_id))
		           FROM bookings b
		           WHERE b.promo_id = p.promo_id
		             AND b.status NOT IN ('cancelled', 'cancelled_by_therapist', 'cancelled_by_client')
		       ), 0) AS current_uses,
		       p.days_of_week, p.start_time, p.end_time, p.is_public, p.deleted_at,
		       p.created_at, p.updated_at
		FROM promotions p
		WHERE (p.valid_from IS NULL OR p.valid_from <= $1)
		  AND (p.valid_until IS NULL OR p.valid_until >= $1)
		  AND p.deleted_at IS NULL` + visibility + `
		ORDER BY p.created_at DESC
    `

	rows, err := r.db.Query(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPromotions(rows)
}

func (r *promotionRepoImpl) ListAll(ctx context.Context) ([]model.Promotion, error) {
	// current_uses is reported from the bookings that actually carry the promo,
	// not from the promotions.current_uses counter. That counter only ever goes
	// up at redemption time: cancelling a booking does not give the use back,
	// and the admin edit path that attaches or clears a voucher never touches
	// it at all, so it drifts away from reality as soon as staff touch a
	// booking. The bookings table is the only record that stays true.
	//
	// A group booking is ONE redemption: the promo_id is copied onto every
	// child booking, so counting rows would triple a three-guest group.
	query := `
        SELECT p.promo_id, p.code, p.discount_percentage, p.discount_amount, p.applies_to,
               p.valid_from, p.valid_until, p.max_uses,
               COALESCE(b.uses, 0) AS current_uses,
               p.days_of_week, p.start_time, p.end_time, p.is_public, p.deleted_at,
               p.created_at, p.updated_at
        FROM promotions p
        LEFT JOIN (
            SELECT promo_id,
                   COUNT(DISTINCT COALESCE('g' || group_id, 'b' || booking_id)) AS uses
            FROM bookings
            WHERE promo_id IS NOT NULL
              AND status NOT IN ('cancelled', 'cancelled_by_therapist', 'cancelled_by_client')
            GROUP BY promo_id
        ) b ON b.promo_id = p.promo_id
        WHERE p.deleted_at IS NULL
        ORDER BY p.created_at DESC
    `

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPromotions(rows)
}

func (r *promotionRepoImpl) ListBookings(ctx context.Context, promoID int64) ([]model.VoucherBooking, error) {
	return r.listVoucherBookings(ctx, "b.promo_id = $1", promoID)
}

func (r *promotionRepoImpl) ListAllVoucherBookings(ctx context.Context) ([]model.VoucherBooking, error) {
	return r.listVoucherBookings(ctx, "b.promo_id IS NOT NULL")
}

func (r *promotionRepoImpl) listVoucherBookings(ctx context.Context, where string, args ...any) ([]model.VoucherBooking, error) {
	query := `
		SELECT p.promo_id,
		       p.code,
		       b.booking_id,
		       COALESCE(NULLIF(b.reference_code, ''), b.booking_id::text),
		       b.group_id,
		       COALESCE(b.guest_name, ''),
		       b.client_id,
		       COALESCE(client.full_name, 'Unknown client'),
		       COALESCE(client.primary_phone, ''),
		       COALESCE(client.primary_email, ''),
		       b.scheduled_start,
		       b.duration_minutes,
		       COALESCE((
		           SELECT jsonb_agg(s.name ORDER BY bs.position)
		           FROM booking_services bs
		           JOIN services s ON s.service_id = bs.service_id
		           WHERE bs.booking_id = b.booking_id
		       ), CASE
		           WHEN primary_service.name IS NULL THEN '[]'::jsonb
		           ELSE jsonb_build_array(primary_service.name)
		       END),
		       COALESCE(NULLIF(therapist.nickname, ''), therapist.full_name, ''),
		       COALESCE(concat_ws(', ',
		           NULLIF(address.street_address, ''),
		           NULLIF(address.barangay, ''),
		           NULLIF(address.city, ''),
		           NULLIF(address.province, ''),
		           NULLIF(address.postal_code, '')
		       ), ''),
		       COALESCE(address.landmark, ''),
		       b.status,
		       COALESCE(b.payment_method, ''),
		       COALESCE(b.booking_source, ''),
		       COALESCE(b.raw_total, 0),
		       COALESCE(b.discount, 0),
		       COALESCE(b.final_total, 0),
		       COALESCE(b.notes, ''),
		       b.created_at
		FROM bookings b
		JOIN promotions p ON p.promo_id = b.promo_id
		LEFT JOIN users client ON client.user_id = b.client_id
		LEFT JOIN users therapist ON therapist.user_id = b.therapist_id
		LEFT JOIN services primary_service ON primary_service.service_id = b.service_id
		LEFT JOIN addresses address ON address.address_id = b.address_id
		WHERE ` + where + `
		ORDER BY COALESCE(b.scheduled_start, b.created_at) DESC, b.booking_id DESC
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]model.VoucherBooking, 0)
	for rows.Next() {
		var booking model.VoucherBooking
		var serviceNamesJSON []byte
		if err := rows.Scan(
			&booking.PromoID,
			&booking.VoucherCode,
			&booking.BookingID,
			&booking.ReferenceCode,
			&booking.GroupID,
			&booking.GuestName,
			&booking.ClientID,
			&booking.ClientName,
			&booking.ClientPhone,
			&booking.ClientEmail,
			&booking.ScheduledStart,
			&booking.DurationMinutes,
			&serviceNamesJSON,
			&booking.TherapistName,
			&booking.Address,
			&booking.Landmark,
			&booking.Status,
			&booking.PaymentMethod,
			&booking.BookingSource,
			&booking.RawTotal,
			&booking.Discount,
			&booking.FinalTotal,
			&booking.Notes,
			&booking.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(serviceNamesJSON, &booking.ServiceNames); err != nil {
			return nil, fmt.Errorf("decode voucher booking services: %w", err)
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bookings, nil
}

func (r *promotionRepoImpl) Update(ctx context.Context, promoID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []interface{}
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, promoID)

	query := fmt.Sprintf("UPDATE promotions SET %s WHERE promo_id = $%d AND deleted_at IS NULL", strings.Join(setClauses, ", "), argIdx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update promotion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *promotionRepoImpl) Delete(ctx context.Context, promoID int64) error {
	query := `UPDATE promotions SET deleted_at = CURRENT_TIMESTAMP WHERE promo_id = $1 AND deleted_at IS NULL`
	cmd, err := r.db.Exec(ctx, query, promoID)
	if err != nil {
		return fmt.Errorf("failed to delete promotion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *promotionRepoImpl) GetByID(ctx context.Context, promoID int64) (*model.Promotion, error) {
	return r.getBy(ctx, "p.promo_id = $1", promoID)
}

func (r *promotionRepoImpl) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	return r.getBy(ctx, "p.code = $1", code)
}

func (r *promotionRepoImpl) getBy(ctx context.Context, where string, arg any) (*model.Promotion, error) {
	query := `
		SELECT p.promo_id, p.code, p.discount_percentage, p.discount_amount, p.applies_to,
		       p.valid_from, p.valid_until, p.max_uses,
		       COALESCE((
		           SELECT COUNT(DISTINCT COALESCE('g' || b.group_id, 'b' || b.booking_id))
		           FROM bookings b
		           WHERE b.promo_id = p.promo_id
		             AND b.status NOT IN ('cancelled', 'cancelled_by_therapist', 'cancelled_by_client')
		       ), 0) AS current_uses,
		       p.days_of_week, p.start_time, p.end_time, p.is_public, p.deleted_at,
		       p.created_at, p.updated_at
		FROM promotions p
		WHERE ` + where + ` AND p.deleted_at IS NULL
    `
	var p model.Promotion
	if err := r.db.QueryRow(ctx, query, arg).Scan(
		&p.PromoID,
		&p.Code,
		&p.DiscountPct,
		&p.DiscountAmount,
		&p.AppliesTo,
		&p.ValidFrom,
		&p.ValidUntil,
		&p.UsageLimit,
		&p.CurrentUses,
		&p.DaysOfWeek,
		&p.StartTime,
		&p.EndTime,
		&p.IsPublic,
		&p.DeletedAt,
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

func (r *promotionRepoImpl) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) {
	// Serialize redemptions for this promotion, then derive availability from
	// bookings. This keeps the limit correct after staff attach/remove vouchers
	// or a booking is cancelled, all of which can make the stored counter stale.
	var maxUses int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max_uses, 0)
		FROM promotions
		WHERE promo_id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, promoID).Scan(&maxUses); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	var currentUses int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT COALESCE('g' || group_id, 'b' || booking_id))
		FROM bookings
		WHERE promo_id = $1
		  AND status NOT IN ('cancelled', 'cancelled_by_therapist', 'cancelled_by_client')
	`, promoID).Scan(&currentUses); err != nil {
		return false, err
	}
	if maxUses > 0 && currentUses >= maxUses {
		return false, nil
	}

	cmd, err := tx.Exec(ctx, `
		UPDATE promotions
		SET current_uses = $2
		WHERE promo_id = $1
	`, promoID, currentUses+1)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *promotionRepoImpl) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) {
	// Try to select existing usage for update
	var timesUsed int
	err := tx.QueryRow(ctx, `SELECT times_used FROM user_promotions WHERE user_id=$1 AND promo_id=$2 FOR UPDATE`, userID, promoID).Scan(&timesUsed)
	if err != nil {
		if err == pgx.ErrNoRows {
			// insert new row
			_, err := tx.Exec(ctx, `INSERT INTO user_promotions (user_id, promo_id, times_used) VALUES ($1,$2,1)`, userID, promoID)
			if err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}
	// update existing
	_, err = tx.Exec(ctx, `UPDATE user_promotions SET times_used = times_used + 1 WHERE user_id=$1 AND promo_id=$2`, userID, promoID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// scanPromotions is a helper to scan promotion rows
func scanPromotions(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.Promotion, error) {
	var out []model.Promotion
	for rows.Next() {
		var p model.Promotion
		if err := rows.Scan(
			&p.PromoID,
			&p.Code,
			&p.DiscountPct,
			&p.DiscountAmount,
			&p.AppliesTo,
			&p.ValidFrom,
			&p.ValidUntil,
			&p.UsageLimit,
			&p.CurrentUses,
			&p.DaysOfWeek,
			&p.StartTime,
			&p.EndTime,
			&p.IsPublic,
			&p.DeletedAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
