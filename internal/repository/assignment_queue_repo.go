package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// AssignmentQueueRepository manages the durable queue for unassigned bookings.
type QueueItem struct {
    BookingID     int64
    Attempts      int
    NextAttempt   *time.Time
    WorkflowState string
    WorkflowData  map[string]interface{}
}

type AssignmentQueueRepository interface {
	Enqueue(ctx context.Context, bookingID int64) error
	EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error
	EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error
	// DequeueBatch returns items that are due (next_attempt_at <= now)
	DequeueBatch(ctx context.Context, limit int) ([]QueueItem, error)
	Remove(ctx context.Context, bookingID int64) error
	// IncrementAttempt increments attempts and sets next_attempt_at
	IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error
	// UpdateWorkflowState updates the durable state of a booking assignment
	UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error
}

type assignmentQueueRepoImpl struct {
	db db.DBTX
}

func NewAssignmentQueueRepository(db db.DBTX) AssignmentQueueRepository {
	return &assignmentQueueRepoImpl{db: db}
}

func (r *assignmentQueueRepoImpl) Enqueue(ctx context.Context, bookingID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO booking_assignment_queue (booking_id, enqueued_at, attempts, next_attempt_at, workflow_state, workflow_data)
		VALUES ($1, $2, 0, $2, 'init', '{}')
		ON CONFLICT (booking_id) DO NOTHING
	`, bookingID, time.Now())
	return err
}

func (r *assignmentQueueRepoImpl) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO booking_assignment_queue (booking_id, enqueued_at, attempts, next_attempt_at, workflow_state, workflow_data)
		VALUES ($1, $2, 0, $2, 'init', '{}')
		ON CONFLICT (booking_id) DO NOTHING
	`, bookingID, time.Now())
	return err
}

func (r *assignmentQueueRepoImpl) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error {
	if len(bookingIDs) == 0 {
		return nil
	}
	
	now := time.Now()
	// Using unnest for bulk insert
	_, err := tx.Exec(ctx, `
		INSERT INTO booking_assignment_queue (booking_id, enqueued_at, attempts, next_attempt_at, workflow_state, workflow_data)
		SELECT UNNEST($1::bigint[]), $2, 0, $2, 'init', '{}'
		ON CONFLICT (booking_id) DO NOTHING
	`, bookingIDs, now)
	return err
}

func (r *assignmentQueueRepoImpl) DequeueBatch(ctx context.Context, limit int) ([]QueueItem, error) {
    rows, err := r.db.Query(ctx, `
        SELECT booking_id, attempts, next_attempt_at, COALESCE(workflow_state, 'init'), COALESCE(workflow_data, '{}')
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
        if err := rows.Scan(&it.BookingID, &it.Attempts, &na, &it.WorkflowState, &it.WorkflowData); err != nil {
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

func (r *assignmentQueueRepoImpl) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
    _, err := r.db.Exec(ctx, `
        UPDATE booking_assignment_queue
        SET workflow_state = $1, workflow_data = $2, updated_at = CURRENT_TIMESTAMP
        WHERE booking_id = $3
    `, state, data, bookingID)
    return err
}
