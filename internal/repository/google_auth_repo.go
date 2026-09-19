package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var ErrGoogleAuthRecordNotFound = errors.New("google auth record not found")

type GoogleAuthRepository interface {
	FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
	FindUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateGoogleUserAndIdentity(ctx context.Context, user model.User, providerKey string) (int, error)
	LinkGoogleIdentity(ctx context.Context, userID int, providerKey string) error
}

type GoogleAuthRepo struct {
	db db.DBTX
}

func NewGoogleAuthRepository(database db.DBTX) GoogleAuthRepository {
	return &GoogleAuthRepo{db: database}
}

func (r *GoogleAuthRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var identity model.UserAuthIdentity
	err := r.db.QueryRow(ctx, `
		SELECT i.identity_id, i.user_id, i.provider, i.provider_key,
		       COALESCE(i.password_hash, ''), i.is_verified, i.created_at
		FROM user_auth_identities i
		JOIN users u ON u.user_id = i.user_id
		WHERE i.provider = $1 AND i.provider_key = $2 AND u.deleted_at IS NULL
	`, provider, key).Scan(
		&identity.IdentityID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderKey,
		&identity.PasswordHash,
		&identity.IsVerified,
		&identity.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGoogleAuthRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find google identity: %w", err)
	}
	return &identity, nil
}

func (r *GoogleAuthRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return r.findUser(ctx, "u.user_id = $1", userID)
}

func (r *GoogleAuthRepo) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.findUser(ctx, "LOWER(u.primary_email) = $1", strings.ToLower(strings.TrimSpace(email)))
}

func (r *GoogleAuthRepo) findUser(ctx context.Context, condition string, value any) (*model.User, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT u.user_id, u.full_name, u.role,
		       COALESCE(u.primary_email, ''), COALESCE(u.primary_phone, ''),
		       COALESCE(u.account_status, 'active'), COALESCE(u.status_reason, ''),
		       COALESCE(u.is_vip, FALSE), COALESCE(u.profile_photo, ''),
		       COALESCE(u.gender, ''), COALESCE(u.emergency_contact_name, ''),
		       COALESCE(u.emergency_contact_phone, ''), u.created_at, u.updated_at
		FROM users u
		WHERE ` + condition + ` AND u.deleted_at IS NULL`

	var user model.User
	err := r.db.QueryRow(ctx, query, value).Scan(
		&user.UserID,
		&user.FullName,
		&user.Role,
		&user.PrimaryEmail,
		&user.PrimaryPhone,
		&user.AccountStatus,
		&user.StatusReason,
		&user.IsVIP,
		&user.ProfilePhoto,
		&user.Gender,
		&user.EmergencyContactName,
		&user.EmergencyContactPhone,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGoogleAuthRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find google auth user: %w", err)
	}
	return &user, nil
}

func (r *GoogleAuthRepo) CreateGoogleUserAndIdentity(ctx context.Context, user model.User, providerKey string) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin google signup: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	err = tx.QueryRow(ctx, `
		INSERT INTO users(full_name, role, primary_email, profile_photo, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING user_id
	`, user.FullName, user.Role, user.PrimaryEmail, user.ProfilePhoto, now).Scan(&user.UserID)
	if err != nil {
		return 0, fmt.Errorf("insert google user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_auth_identities(user_id, provider, provider_key, is_verified, created_at)
		VALUES ($1, 'google', $2, TRUE, $3)
	`, user.UserID, providerKey, now)
	if err != nil {
		return 0, fmt.Errorf("insert google identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit google signup: %w", err)
	}
	return user.UserID, nil
}

func (r *GoogleAuthRepo) LinkGoogleIdentity(ctx context.Context, userID int, providerKey string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_auth_identities(user_id, provider, provider_key, is_verified, created_at)
		VALUES ($1, 'google', $2, TRUE, CURRENT_TIMESTAMP)
	`, userID, providerKey)
	if err != nil {
		return fmt.Errorf("link google identity: %w", err)
	}
	return nil
}
