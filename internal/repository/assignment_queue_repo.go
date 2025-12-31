package repository

import (
	"context"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// AssignmentQueueRepository manages the durable queue for unassigned bookings.
type QueueItem struct {
    BookingID   int64
    Attempts    int
    NextAttempt *time.Time
}

type AssignmentQueueRepository interface {
    Enqueue(ctx context.Context, bookingID int64) error
    // DequeueBatch returns items that are due (next_attempt_at <= now)
    DequeueBatch(ctx context.Context, limit int) ([]QueueItem, error)
    Remove(ctx context.Context, bookingID int64) error
    // IncrementAttempt increments attempts and sets next_attempt_at
    IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error
}

type assignmentQueueRepoImpl struct {
    db db.DBTX
}

func NewAssignmentQueueRepository(db db.DBTX) AssignmentQueueRepository {
    return &assignmentQueueRepoImpl{db: db}
}

func (r *assignmentQueueRepoImpl) Enqueue(ctx context.Context, bookingID int64) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO booking_assignment_queue (booking_id, enqueued_at, attempts, next_attempt_at)
        VALUES ($1, $2, 0, $2)
        ON CONFLICT (booking_id) DO NOTHING
    `, bookingID, time.Now())
    return err
}

func (r *assignmentQueueRepoImpl) DequeueBatch(ctx context.Context, limit int) ([]QueueItem, error) {
    rows, err := r.db.Query(ctx, `
        SELECT booking_id, attempts, next_attempt_at
        FROM booking_assignment_queue
        WHERE next_attempt_at <= CURRENT_TIMESTAMP
        ORDER BY enqueued_at ASC
        LIMIT $1
    `, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []QueueItem
    for rows.Next() {
        var it QueueItem
        var na *time.Time
        if err := rows.Scan(&it.BookingID, &it.Attempts, &na); err != nil {
            return nil, err
        }
        it.NextAttempt = na
        out = append(out, it)
    }
    return out, nil
}

func (r *assignmentQueueRepoImpl) Remove(ctx context.Context, bookingID int64) error {
    _, err := r.db.Exec(ctx, `DELETE FROM booking_assignment_queue WHERE booking_id = $1`, bookingID)
    return err
}

func (r *assignmentQueueRepoImpl) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
    _, err := r.db.Exec(ctx, `
        UPDATE booking_assignment_queue
        SET attempts = $1, last_attempt_at = $2, next_attempt_at = $3
        WHERE booking_id = $4
    `, attempts, time.Now(), nextAttempt, bookingID)
    return err
}
