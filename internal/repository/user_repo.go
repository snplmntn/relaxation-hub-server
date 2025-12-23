package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type UserRepository interface {
	CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
	UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error
	ListUsers(ctx context.Context, role string) ([]model.User, error)
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
		INSERT INTO user_auth_identities(user_id, provider, provider_key, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err = transaction.Exec(ctx, queryIdentity,
		user.UserID, identity.Provider, identity.ProviderKey, identity.PasswordHash, identity.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert user auth identity: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *UserRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	query := `
		SELECT i.identity_id, i.user_id, i.provider, i.provider_key, i.password_hash, i.is_verified, i.created_at
		FROM user_auth_identities i
		JOIN users u ON i.user_id = u.user_id
		WHERE i.provider = $1 AND i.provider_key = $2 AND u.deleted_at IS NULL`
	row := r.db.QueryRow(ctx, query, provider, key)

	var identity model.UserAuthIdentity
	err := row.Scan(&identity.IdentityID, &identity.UserID, &identity.Provider,
		&identity.ProviderKey, &identity.PasswordHash, &identity.IsVerified, &identity.CreatedAt)
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
	SELECT user_id, full_name, role, 
		COALESCE(primary_email, ''), COALESCE(primary_phone, ''), 
		COALESCE(profile_photo, ''), COALESCE(gender, ''), 
		COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, ''), 
		created_at, updated_at
	FROM users
	WHERE user_id = $1 AND deleted_at IS NULL`
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

func (r *UserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
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
	args = append(args, userID)

	query := fmt.Sprintf("UPDATE users SET %s WHERE user_id = $%d AND deleted_at IS NULL", strings.Join(setClauses, ", "), argIdx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *UserRepo) ListUsers(ctx context.Context, role string) ([]model.User, error) {
	var rows pgx.Rows
	var err error

	if role == "" {
		query := `
		SELECT user_id, full_name, role,
			COALESCE(primary_email, ''), COALESCE(primary_phone, ''),
			COALESCE(profile_photo, ''), COALESCE(gender, ''),
			COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, ''),
			created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC` 
		rows, err = r.db.Query(ctx, query)
	} else {
		query := `
		SELECT user_id, full_name, role,
			COALESCE(primary_email, ''), COALESCE(primary_phone, ''),
			COALESCE(profile_photo, ''), COALESCE(gender, ''),
			COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, ''),
			created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL AND role = $1
		ORDER BY created_at DESC` 
		rows, err = r.db.Query(ctx, query, role)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.UserID, &u.FullName, &u.Role, &u.PrimaryEmail, &u.PrimaryPhone, &u.ProfilePhoto, &u.Gender, &u.EmergencyContactName, &u.EmergencyContactPhone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}
