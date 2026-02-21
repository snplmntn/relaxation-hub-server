package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// NotificationRepository manages notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error)
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
        VALUES ($1,$2,$3,$4,$5)
        RETURNING notification_id, is_read, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		n.UserID,
		n.Type,
		n.Title,
		n.Message,
		n.Data,
	).Scan(&n.NotificationID, &n.IsRead, &n.CreatedAt, &n.UpdatedAt)
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
