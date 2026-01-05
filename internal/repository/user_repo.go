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

type UserRepository interface {
	CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
	UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error
	ListUsers(ctx context.Context, role string) ([]model.User, error)
	BlockUser(ctx context.Context, blockerID, blockedID int64) error
	UnblockUser(ctx context.Context, blockerID, blockedID int64) error
	IsBlocked(ctx context.Context, userA, userB int64) (bool, error)
	GetBlockList(ctx context.Context, userID int64) ([]BlockedUserEntry, error)
	// FCM token management for push notifications
	UpdateFCMToken(ctx context.Context, userID int64, token string) error
	GetFCMToken(ctx context.Context, userID int64) (*string, error)
	// Batch fetching methods for optimization
	GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*UserInfo, error)
	GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*TherapistInfo, error)
	// Favorite Therapists
	AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error
	RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error
	ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error)
	IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error)
	// BanUserSystem bans a user by the system (sets account_status to 'banned')
	BanUserSystem(ctx context.Context, userID int64, reason string) error
}

// UserInfo represents basic user info for booking enrichment
type UserInfo struct {
	UserID int64
	Name   string
	Phone  string
	Photo  string
	Gender string
}

// TherapistInfo represents therapist info including rating
type TherapistInfo struct {
	UserInfo
	Rating *float64
}


// BlockedUserEntry represents a blocked user with enriched info
type BlockedUserEntry struct {
	UserID       int64  `json:"user_id"`
	FullName     string `json:"full_name"`
	ProfilePhoto string `json:"profile_photo"`
	BlockedAt    string `json:"blocked_at"`
}

type UserRepo struct {
	db db.DBTX
}

func NewUserRepository(db db.DBTX) UserRepository {
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
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

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
		COALESCE(account_status, 'active'),
		COALESCE(profile_photo, ''), COALESCE(gender, ''), 
		COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, ''), 
		created_at, updated_at
	FROM users
	WHERE user_id = $1 AND deleted_at IS NULL`
	row := r.db.QueryRow(ctx, query, userID)

	var user model.User
	err := row.Scan(&user.UserID, &user.FullName, &user.Role, &user.PrimaryEmail,
		&user.PrimaryPhone, &user.AccountStatus, &user.ProfilePhoto, &user.Gender,
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
			COALESCE(account_status, 'active'),
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
			COALESCE(account_status, 'active'),
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
		if err := rows.Scan(&u.UserID, &u.FullName, &u.Role, &u.PrimaryEmail, &u.PrimaryPhone, &u.AccountStatus, &u.ProfilePhoto, &u.Gender, &u.EmergencyContactName, &u.EmergencyContactPhone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepo) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `INSERT INTO user_blocks (blocker_user_id, blocked_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, blockerID, blockedID)
	return err
}

func (r *UserRepo) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `DELETE FROM user_blocks WHERE blocker_user_id = $1 AND blocked_user_id = $2`
	_, err := r.db.Exec(ctx, query, blockerID, blockedID)
	return err
}

func (r *UserRepo) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT EXISTS (
		SELECT 1 FROM user_blocks 
		WHERE (blocker_user_id = $1 AND blocked_user_id = $2) 
		   OR (blocker_user_id = $2 AND blocked_user_id = $1)
	)`
	var exists bool
	err := r.db.QueryRow(ctx, query, userA, userB).Scan(&exists)
	return exists, err
}

func (r *UserRepo) GetBlockList(ctx context.Context, userID int64) ([]BlockedUserEntry, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT ub.blocked_user_id, COALESCE(u.full_name, 'Unknown'), COALESCE(u.profile_photo, ''), ub.created_at
		FROM user_blocks ub
		LEFT JOIN users u ON ub.blocked_user_id = u.user_id
		WHERE ub.blocker_user_id = $1
		ORDER BY ub.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlockedUserEntry
	for rows.Next() {
		var e BlockedUserEntry
		var blockedAt time.Time
		if err := rows.Scan(&e.UserID, &e.FullName, &e.ProfilePhoto, &blockedAt); err != nil {
			return nil, err
		}
		e.BlockedAt = blockedAt.Format(time.RFC3339)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetUserInfoBatch fetches user info for multiple user IDs in a single query
func (r *UserRepo) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*UserInfo, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(userIDs) == 0 {
		return map[int64]*UserInfo{}, nil
	}

	query := `
		SELECT user_id, COALESCE(full_name, ''), COALESCE(primary_phone, ''), 
		       COALESCE(profile_photo, ''), COALESCE(gender, '')
		FROM users 
		WHERE user_id = ANY($1) AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query user info batch: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]*UserInfo)
	for rows.Next() {
		var info UserInfo
		if err := rows.Scan(&info.UserID, &info.Name, &info.Phone, &info.Photo, &info.Gender); err != nil {
			return nil, fmt.Errorf("failed to scan user info: %w", err)
		}
		result[info.UserID] = &info
	}
	return result, rows.Err()
}

// GetTherapistInfoBatch fetches therapist info including ratings for multiple IDs
func (r *UserRepo) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*TherapistInfo, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(therapistIDs) == 0 {
		return map[int64]*TherapistInfo{}, nil
	}

	query := `
		SELECT u.user_id, COALESCE(u.full_name, ''), COALESCE(u.primary_phone, ''), 
		       COALESCE(u.profile_photo, ''), COALESCE(u.gender, ''), tp.avg_rating
		FROM users u
		LEFT JOIN therapist_profiles tp ON u.user_id = tp.therapist_id AND tp.deleted_at IS NULL
		WHERE u.user_id = ANY($1) AND u.deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, therapistIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query therapist info batch: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]*TherapistInfo)
	for rows.Next() {
		var info TherapistInfo
		var rating *float64
		if err := rows.Scan(&info.UserID, &info.Name, &info.Phone, &info.Photo, &info.Gender, &rating); err != nil {
			return nil, fmt.Errorf("failed to scan therapist info: %w", err)
		}
		info.Rating = rating
		result[info.UserID] = &info
	}
	return result, rows.Err()
}

// UpdateFCMToken updates the FCM token for a user
func (r *UserRepo) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `UPDATE users SET fcm_token = $1, updated_at = CURRENT_TIMESTAMP WHERE user_id = $2 AND deleted_at IS NULL`
	cmd, err := r.db.Exec(ctx, query, token, userID)
	if err != nil {
		return fmt.Errorf("failed to update FCM token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetFCMToken retrieves the FCM token for a user
func (r *UserRepo) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT fcm_token FROM users WHERE user_id = $1 AND deleted_at IS NULL`
	var token *string
	err := r.db.QueryRow(ctx, query, userID).Scan(&token)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get FCM token: %w", err)
	}
	return token, nil
}

// AddFavoriteTherapist adds a therapist to the user's favorites
func (r *UserRepo) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `INSERT INTO favorite_therapists (user_id, therapist_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, userID, therapistID)
	return err
}

// RemoveFavoriteTherapist removes a therapist from the user's favorites
func (r *UserRepo) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `DELETE FROM favorite_therapists WHERE user_id = $1 AND therapist_id = $2`
	_, err := r.db.Exec(ctx, query, userID, therapistID)
	return err
}

// ListFavoriteTherapists returns a list of favorite therapists for a user
func (r *UserRepo) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT u.user_id, u.full_name, u.role,
			COALESCE(u.primary_email, ''), COALESCE(u.primary_phone, ''),
			COALESCE(u.account_status, 'active'),
			COALESCE(u.profile_photo, ''), COALESCE(u.gender, ''),
			COALESCE(u.emergency_contact_name, ''), COALESCE(u.emergency_contact_phone, ''),
			u.created_at, u.updated_at
		FROM users u
		JOIN favorite_therapists ft ON u.user_id = ft.therapist_id
		WHERE ft.user_id = $1 AND u.deleted_at IS NULL
		ORDER BY ft.created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorite therapists: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.UserID, &u.FullName, &u.Role, &u.PrimaryEmail, &u.PrimaryPhone, &u.AccountStatus, &u.ProfilePhoto, &u.Gender, &u.EmergencyContactName, &u.EmergencyContactPhone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan favorite therapist: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

// IsTherapistFavorite checks if a therapist is in the user's favorites
func (r *UserRepo) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `SELECT EXISTS (SELECT 1 FROM favorite_therapists WHERE user_id = $1 AND therapist_id = $2)`
	var exists bool
	err := r.db.QueryRow(ctx, query, userID, therapistID).Scan(&exists)
	return exists, err
}

// BanUserSystem sets account_status to 'banned' for system-triggered bans
func (r *UserRepo) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Update account_status to banned
	cmd, err := r.db.Exec(ctx, `
		UPDATE users 
		SET account_status = 'banned', updated_at = CURRENT_TIMESTAMP 
		WHERE user_id = $1 AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
