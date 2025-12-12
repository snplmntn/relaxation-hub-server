package repository

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ReferralRepository manages referrals and rewards.
type ReferralRepository interface {
	Create(ctx context.Context, r *model.Referral) error
	GetByCode(ctx context.Context, code string) (*model.Referral, error)
	GetByID(ctx context.Context, referralID int64) (*model.Referral, error)
	GetByReferrer(ctx context.Context, referrerID int64) ([]model.Referral, error)
	UpdateStatus(ctx context.Context, referralID int64, status string) error
	CreateReward(ctx context.Context, rw *model.ReferralReward) error
	GetRewardsByUser(ctx context.Context, userID int64) ([]model.ReferralReward, error)
	RedeemReward(ctx context.Context, rewardID, userID int64) error
}

type referralRepoImpl struct {
	db *pgxpool.Pool
}

func NewReferralRepository(db *pgxpool.Pool) ReferralRepository {
	return &referralRepoImpl{db: db}
}

func (r *referralRepoImpl) Create(ctx context.Context, ref *model.Referral) error {
	query := `
        INSERT INTO referrals (referrer_id, referred_id, referral_code, status)
        VALUES ($1,$2,$3,$4)
        RETURNING referral_id, reward_earned, created_at
    `
	return r.db.QueryRow(ctx, query,
		ref.ReferrerID,
		ref.ReferredID,
		ref.ReferralCode,
		ref.Status,
	).Scan(&ref.ReferralID, &ref.RewardEarned, &ref.CreatedAt)
}

func (r *referralRepoImpl) GetByCode(ctx context.Context, code string) (*model.Referral, error) {
	query := `
		SELECT referral_id, referrer_id, referred_id, referral_code, status, reward_earned, created_at, completed_at
		FROM referrals
		WHERE referral_code = $1
	`
	var ref model.Referral
	if err := r.db.QueryRow(ctx, query, code).Scan(
		&ref.ReferralID,
		&ref.ReferrerID,
		&ref.ReferredID,
		&ref.ReferralCode,
		&ref.Status,
		&ref.RewardEarned,
		&ref.CreatedAt,
		&ref.CompletedAt,
	); err != nil {
		return nil, err
	}
	return &ref, nil
}

// GetByID retrieves a referral by its ID
func (r *referralRepoImpl) GetByID(ctx context.Context, referralID int64) (*model.Referral, error) {
	query := `
		SELECT referral_id, referrer_id, referred_id, referral_code, status, reward_earned, created_at, completed_at
		FROM referrals
		WHERE referral_id = $1
	`
	var ref model.Referral
	if err := r.db.QueryRow(ctx, query, referralID).Scan(
		&ref.ReferralID,
		&ref.ReferrerID,
		&ref.ReferredID,
		&ref.ReferralCode,
		&ref.Status,
		&ref.RewardEarned,
		&ref.CreatedAt,
		&ref.CompletedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("referral not found")
		}
		return nil, err
	}
	return &ref, nil
}

func (r *referralRepoImpl) GetByReferrer(ctx context.Context, referrerID int64) ([]model.Referral, error) {
	query := `
        SELECT referral_id, referrer_id, referred_id, referral_code, status, reward_earned, created_at, completed_at
        FROM referrals
        WHERE referrer_id = $1
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []model.Referral
	for rows.Next() {
		var ref model.Referral
		if err := rows.Scan(&ref.ReferralID, &ref.ReferrerID, &ref.ReferredID, &ref.ReferralCode, &ref.Status, &ref.RewardEarned, &ref.CreatedAt, &ref.CompletedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *referralRepoImpl) UpdateStatus(ctx context.Context, referralID int64, status string) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE referrals
        SET status = $1,
            completed_at = CASE WHEN $1 = 'completed' THEN CURRENT_TIMESTAMP ELSE completed_at END,
            reward_earned = CASE WHEN $1 = 'completed' THEN TRUE ELSE reward_earned END
        WHERE referral_id = $2
    `, status, referralID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *referralRepoImpl) CreateReward(ctx context.Context, rw *model.ReferralReward) error {
	query := `
        INSERT INTO referral_rewards (referral_id, user_id, reward_type, reward_amount, status, expires_at)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING reward_id, created_at
    `
	return r.db.QueryRow(ctx, query,
		rw.ReferralID,
		rw.UserID,
		rw.RewardType,
		rw.RewardAmount,
		rw.Status,
		rw.ExpiresAt,
	).Scan(&rw.RewardID, &rw.CreatedAt)
}

func (r *referralRepoImpl) GetRewardsByUser(ctx context.Context, userID int64) ([]model.ReferralReward, error) {
	query := `
        SELECT reward_id, referral_id, user_id, reward_type, reward_amount, status, expires_at, redeemed_at, created_at
        FROM referral_rewards
        WHERE user_id = $1
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rewards []model.ReferralReward
	for rows.Next() {
		var rw model.ReferralReward
		if err := rows.Scan(&rw.RewardID, &rw.ReferralID, &rw.UserID, &rw.RewardType, &rw.RewardAmount, &rw.Status, &rw.ExpiresAt, &rw.RedeemedAt, &rw.CreatedAt); err != nil {
			return nil, err
		}
		rewards = append(rewards, rw)
	}
	return rewards, rows.Err()
}

func (r *referralRepoImpl) RedeemReward(ctx context.Context, rewardID, userID int64) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE referral_rewards
        SET status = 'redeemed',
            redeemed_at = CURRENT_TIMESTAMP
        WHERE reward_id = $1 AND user_id = $2 AND status = 'pending'
    `, rewardID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GenerateReferralCode creates a unique referral code.
func GenerateReferralCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rnd.Intn(len(charset))]
	}
	return fmt.Sprintf("REF-%s", string(code))
}
