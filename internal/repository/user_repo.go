package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type UserRepository interface {
	CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
}

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &UserRepo{db: db}
}

func (r *UserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	transaction, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin database transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	query := `
		INSERT INTO users(full_name, role, primary_email, primary_phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING user_id`
	err = transaction.QueryRow(ctx, query, user.FullName, user.Role, user.PrimaryEmail, user.PrimaryPhone,
		user.CreatedAt, user.UpdatedAt).Scan(&user.UserID)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	queryIdentity := `
		INSERT INTO user_auth_identities(user_id, provider, provider_key, password, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err = transaction.Exec(ctx, queryIdentity,
		user.UserID, identity.Provider, identity.ProviderKey, identity.Password, identity.CreatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to insert user auth identity: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

func (r *UserRepo) 	FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	query := `
		SELECT identity_id, user_id, provider, provider_key, password, is_verified, created_at
		FROM user_auth_identities
		WHERE provider = $1 AND provider_key = $2`
	row := r.db.QueryRow(ctx, query, provider, key)

	var identity model.UserAuthIdentity
	err := row.Scan(&identity.IdentityID, &identity.UserID, &identity.Provider,
		&identity.ProviderKey, &identity.Password, &identity.IsVerified, &identity.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("identity not found")
		}
		return nil, fmt.Errorf("failed to find identify: %w", err)
	}

	return &identity, nil
}

func (r *UserRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	query := `
	SELECT user_id, full_name, role, primary_email, primary_phone, 
		COALESCE(profile_photo, ''), COALESCE(gender, ''), 
		COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, ''), 
		created_at, updated_at
	FROM users
	WHERE user_id = $1`
	row := r.db.QueryRow(ctx, query, userID)

	var user model.User
	err := row.Scan(&user.UserID, &user.FullName, &user.Role, &user.PrimaryEmail,
		&user.PrimaryPhone, &user.ProfilePhoto, &user.Gender,
		&user.EmergencyContactName, &user.EmergencyContactPhone, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	return &user, nil
}
	