package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// NotificationRepository manages notifications.
type NotificationRepository interface {
    Create(ctx context.Context, n *model.Notification) error
    ListByUser(ctx context.Context, userID int64) ([]model.Notification, error)
    MarkAsRead(ctx context.Context, notificationID, userID int64) error
}

type notificationRepoImpl struct {
    db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) NotificationRepository {
    return &notificationRepoImpl{db: db}
}

func (r *notificationRepoImpl) Create(ctx context.Context, n *model.Notification) error {
    query := `
        INSERT INTO notifications (user_id, type, title, message)
        VALUES ($1,$2,$3,$4)
        RETURNING notification_id, is_read, created_at, updated_at
    `
    return r.db.QueryRow(ctx, query,
        n.UserID,
        n.Type,
        n.Title,
        n.Message,
    ).Scan(&n.NotificationID, &n.IsRead, &n.CreatedAt, &n.UpdatedAt)
}

func (r *notificationRepoImpl) ListByUser(ctx context.Context, userID int64) ([]model.Notification, error) {
    query := `
        SELECT notification_id, user_id, type, title, message, data, is_read, read_at, created_at, updated_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
    `
    rows, err := r.db.Query(ctx, query, userID)
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
    return out, nil
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
