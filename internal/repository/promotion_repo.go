package repository

import (
	"context"
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
	ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error)
	GetByCode(ctx context.Context, code string) (*model.Promotion, error)
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
            days_of_week, start_time, end_time
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
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
	).Scan(&p.PromoID, &p.CurrentUses, &p.CreatedAt, &p.UpdatedAt)
}

func (r *promotionRepoImpl) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) {
	query := `
        SELECT promo_id, code, discount_percentage, discount_amount, applies_to, valid_from, valid_until, max_uses,
               current_uses, days_of_week, start_time, end_time, deleted_at, created_at, updated_at
        FROM promotions
        WHERE (valid_from IS NULL OR valid_from <= $1)
          AND (valid_until IS NULL OR valid_until >= $1)
          AND deleted_at IS NULL
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPromotions(rows)
}

func (r *promotionRepoImpl) ListAll(ctx context.Context) ([]model.Promotion, error) {
	query := `
        SELECT promo_id, code, discount_percentage, discount_amount, applies_to, valid_from, valid_until, max_uses,
               current_uses, days_of_week, start_time, end_time, deleted_at, created_at, updated_at
        FROM promotions
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPromotions(rows)
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

func (r *promotionRepoImpl) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	query := `
        SELECT promo_id, code, discount_percentage, discount_amount, applies_to, valid_from, valid_until, max_uses,
               current_uses, days_of_week, start_time, end_time, deleted_at, created_at, updated_at
        FROM promotions
        WHERE code = $1 AND deleted_at IS NULL
    `
	var p model.Promotion
	if err := r.db.QueryRow(ctx, query, code).Scan(
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
	cmd, err := tx.Exec(ctx, `
		UPDATE promotions
		SET current_uses = current_uses + 1
		WHERE promo_id = $1 AND (max_uses IS NULL OR current_uses < max_uses)
	`, promoID)
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
