package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type BookingReferralRepository interface {
	CreateTx(ctx context.Context, tx pgx.Tx, referral *model.BookingReferral) error
	ListSummaryTotals(ctx context.Context, startDate, endDate time.Time) ([]model.BookingReferralSummaryTotal, error)
	ListSummarySeries(ctx context.Context, startDate, endDate time.Time, bucket string) ([]model.BookingReferralSummaryPoint, error)
}

type bookingReferralRepoImpl struct {
	db db.DBTX
}

func NewBookingReferralRepository(database db.DBTX) BookingReferralRepository {
	return &bookingReferralRepoImpl{db: database}
}

func (r *bookingReferralRepoImpl) CreateTx(ctx context.Context, tx pgx.Tx, referral *model.BookingReferral) error {
	query := `
		INSERT INTO booking_referrals (booking_id, source, other_notes, created_by_user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (booking_id) DO UPDATE SET
			source = EXCLUDED.source,
			other_notes = EXCLUDED.other_notes,
			created_by_user_id = EXCLUDED.created_by_user_id
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, referral.BookingID, referral.Source, referral.OtherNotes, referral.CreatedByUserID).Scan(&referral.CreatedAt)
}

func (r *bookingReferralRepoImpl) ListSummaryTotals(ctx context.Context, startDate, endDate time.Time) ([]model.BookingReferralSummaryTotal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT source, COUNT(*)::BIGINT AS total
		FROM booking_referrals
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY source
		ORDER BY total DESC, source ASC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to list referral totals: %w", err)
	}
	defer rows.Close()

	out := make([]model.BookingReferralSummaryTotal, 0)
	for rows.Next() {
		var row model.BookingReferralSummaryTotal
		if err := rows.Scan(&row.Source, &row.Count); err != nil {
			return nil, fmt.Errorf("failed to scan referral total row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating referral totals: %w", err)
	}
	return out, nil
}

func (r *bookingReferralRepoImpl) ListSummarySeries(ctx context.Context, startDate, endDate time.Time, bucket string) ([]model.BookingReferralSummaryPoint, error) {
	dateTrunc := "day"
	if bucket == "week" {
		dateTrunc = "week"
	}

	query := fmt.Sprintf(`
		SELECT date_trunc('%s', created_at)::date AS period_start, source, COUNT(*)::BIGINT AS total
		FROM booking_referrals
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY period_start, source
		ORDER BY period_start ASC, source ASC
	`, dateTrunc)

	rows, err := r.db.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to list referral series: %w", err)
	}
	defer rows.Close()

	out := make([]model.BookingReferralSummaryPoint, 0)
	for rows.Next() {
		var row model.BookingReferralSummaryPoint
		if err := rows.Scan(&row.PeriodStart, &row.Source, &row.Count); err != nil {
			return nil, fmt.Errorf("failed to scan referral series row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating referral series: %w", err)
	}
	return out, nil
}
