package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

type AccountSecurityRepository interface {
	GetEmailPasswordHash(ctx context.Context, userID int64) (string, error)
	UpdateEmailPasswordHash(ctx context.Context, userID int64, passwordHash string) error
	UpdateStaffEmailPasswordHash(ctx context.Context, userID int64, passwordHash string) error
	DeleteClientAccount(ctx context.Context, userID int64) error
}

type accountSecurityRepository struct {
	db db.DBTX
}

func NewAccountSecurityRepository(db db.DBTX) AccountSecurityRepository {
	return &accountSecurityRepository{db: db}
}

func (r *accountSecurityRepository) GetEmailPasswordHash(ctx context.Context, userID int64) (string, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var passwordHash string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(i.password_hash, '')
		FROM user_auth_identities i
		JOIN users u ON u.user_id = i.user_id
		WHERE i.user_id = $1
		  AND i.provider = 'email'
		  AND u.deleted_at IS NULL
	`, userID).Scan(&passwordHash)
	if err != nil {
		return "", err
	}
	return passwordHash, nil
}

func (r *accountSecurityRepository) UpdateEmailPasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	result, err := r.db.Exec(ctx, `
		UPDATE user_auth_identities
		SET password_hash = $2
		WHERE user_id = $1 AND provider = 'email'
	`, userID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *accountSecurityRepository) UpdateStaffEmailPasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	result, err := r.db.Exec(ctx, `
		UPDATE user_auth_identities AS identity
		SET password_hash = $2
		FROM users AS target
		WHERE identity.user_id = $1
		  AND identity.provider = 'email'
		  AND target.user_id = identity.user_id
		  AND target.role IN ('admin', 'super_admin')
		  AND target.deleted_at IS NULL
	`, userID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *accountSecurityRepository) DeleteClientAccount(ctx context.Context, userID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin account deletion: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE users
		SET full_name = 'Deleted User',
		    primary_email = NULL,
		    primary_phone = NULL,
		    profile_photo = NULL,
		    gender = NULL,
		    emergency_contact_name = NULL,
		    emergency_contact_phone = NULL,
		    notification_preferences = '{}'::jsonb,
		    account_status = 'inactive',
		    status_reason = 'Self-service account deletion',
		    is_vip = FALSE,
		    fcm_token = NULL,
		    deleted_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND role = 'client' AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("delete client profile: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_auth_identities WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete login identities: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE addresses
		SET label = NULL,
		    street_address = 'Deleted',
		    barangay = NULL,
		    city = 'Deleted',
		    province = NULL,
		    postal_code = NULL,
		    landmark = NULL,
		    latitude = NULL,
		    longitude = NULL,
		    is_default = FALSE,
		    deleted_at = COALESCE(deleted_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("delete saved addresses: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM favorite_therapists WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete favorite therapists: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_blocks WHERE blocker_user_id = $1 OR blocked_user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user blocks: %w", err)
	}

	return tx.Commit(ctx)
}
