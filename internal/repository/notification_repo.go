package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// NotificationRepository manages notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	CreateMany(ctx context.Context, notifications []*model.Notification) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error)
	ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error)
	CountUnreadByUser(ctx context.Context, userID int64) (int, error)
	MarkAsRead(ctx context.Context, notificationID, userID int64) error
	MarkAllAsRead(ctx context.Context, userID int64) error
}

type notificationRepoImpl struct {
	db db.DBTX
}

func NewNotificationRepository(db db.DBTX) NotificationRepository {
	return &notificationRepoImpl{db: db}
}

func (r *notificationRepoImpl) Create(ctx context.Context, n *model.Notification) error {
	query := `
        INSERT INTO notifications (user_id, type, title, message, data)
        VALUES ($1,$2,$3,$4,NULLIF($5, '')::jsonb)
        RETURNING notification_id, is_read, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		n.UserID,
		n.Type,
		n.Title,
		n.Message,
		string(n.Data),
	).Scan(&n.NotificationID, &n.IsRead, &n.CreatedAt, &n.UpdatedAt)
}

func (r *notificationRepoImpl) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	values := make([]string, 0, len(notifications))
	args := make([]any, 0, len(notifications)*5)
	for i, notification := range notifications {
		base := i*5 + 1
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,NULLIF($%d, '')::jsonb)", base, base+1, base+2, base+3, base+4))
		args = append(args, notification.UserID, notification.Type, notification.Title, notification.Message, string(notification.Data))
	}

	query := fmt.Sprintf(`
        INSERT INTO notifications (user_id, type, title, message, data)
        VALUES %s
        RETURNING notification_id, is_read, created_at, updated_at
    `, strings.Join(values, ","))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("create notifications batch: %w", err)
	}
	defer rows.Close()

	returned := 0
	for rows.Next() {
		if returned >= len(notifications) {
			return fmt.Errorf("create notifications batch: returned more rows than inserted notifications")
		}
		if err := rows.Scan(
			&notifications[returned].NotificationID,
			&notifications[returned].IsRead,
			&notifications[returned].CreatedAt,
			&notifications[returned].UpdatedAt,
		); err != nil {
			return fmt.Errorf("create notifications batch: scan returned row %d: %w", returned, err)
		}
		returned++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("create notifications batch: %w", err)
	}
	if returned != len(notifications) {
		return fmt.Errorf("create notifications batch: expected %d returned rows, got %d", len(notifications), returned)
	}
	return nil
}

func (r *notificationRepoImpl) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	// 1. Get total count
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get paginated notifications
	query := `
        SELECT notification_id, user_id, type, title, message, data, is_read, read_at, created_at, updated_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.NotificationID,
			&n.UserID,
			&n.Type,
			&n.Title,
			&n.Message,
			&n.Data,
			&n.IsRead,
			&n.ReadAt,
			&n.CreatedAt,
			&n.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, total, nil
}

func (r *notificationRepoImpl) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	query := `
        SELECT notification_id, user_id, type, title, message, data, is_read, read_at, created_at, updated_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC, notification_id DESC
        LIMIT $2
    `
	args := []any{userID, limit}
	if cursor != nil {
		query = `
            SELECT notification_id, user_id, type, title, message, data, is_read, read_at, created_at, updated_at
            FROM notifications
            WHERE user_id = $1
              AND (created_at < $2 OR (created_at = $2 AND notification_id < $3))
            ORDER BY created_at DESC, notification_id DESC
            LIMIT $4
        `
		args = []any{userID, cursor.CreatedAt, cursor.ID, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.NotificationID,
			&n.UserID,
			&n.Type,
			&n.Title,
			&n.Message,
			&n.Data,
			&n.IsRead,
			&n.ReadAt,
			&n.CreatedAt,
			&n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *notificationRepoImpl) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	var unread int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND is_read = FALSE
	`, userID).Scan(&unread)
	if err != nil {
		return 0, err
	}
	return unread, nil
}

func (r *notificationRepoImpl) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE notifications
        SET is_read = TRUE, read_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
        WHERE notification_id = $1 AND user_id = $2
    `, notificationID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *notificationRepoImpl) MarkAllAsRead(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `
        UPDATE notifications
        SET is_read = TRUE, read_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $1 AND is_read = FALSE
    `, userID)
	return err
}
