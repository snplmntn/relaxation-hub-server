package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type StaffOutTimeRepository interface {
	ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error)
	GetStaffOutTimeTargetUser(ctx context.Context, userID int64) (*model.StaffOutTimeUser, error)
	CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error)
	UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error)
	VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error
}

type staffOutTimeRepoImpl struct {
	db db.DBTX
}

func NewStaffOutTimeRepository(database db.DBTX) StaffOutTimeRepository {
	return &staffOutTimeRepoImpl{db: database}
}

func (r *staffOutTimeRepoImpl) ListStaffOutTimes(ctx context.Context, filter model.StaffOutTimeFilter) ([]model.StaffOutTime, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, staffOutTimeSelectSQL+`
		WHERE sot.voided_at IS NULL
		  AND sot.work_date = $1
		  AND u.deleted_at IS NULL
		  AND u.role IN ('therapist', 'rider', 'admin', 'super_admin')
		  AND ($2 = '' OR u.role = $2)
		  AND ($3 = '' OR u.full_name ILIKE '%' || $3 || '%' OR u.primary_email ILIKE '%' || $3 || '%' OR u.primary_phone ILIKE '%' || $3 || '%')
		ORDER BY u.role, u.full_name, sot.out_time_id`,
		filter.WorkDate.Format("2006-01-02"), strings.TrimSpace(filter.Role), strings.TrimSpace(filter.Search))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.StaffOutTime, 0)
	for rows.Next() {
		var item model.StaffOutTime
		if err := rows.Scan(staffOutTimeScanTargets(&item)...); err != nil {
			return nil, err
		}
		fillStaffOutTimeDate(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *staffOutTimeRepoImpl) GetStaffOutTimeTargetUser(ctx context.Context, userID int64) (*model.StaffOutTimeUser, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	user := model.StaffOutTimeUser{}
	err := r.db.QueryRow(ctx, `
		SELECT user_id, full_name, role
		FROM users
		WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&user.UserID, &user.FullName, &user.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *staffOutTimeRepoImpl) CreateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffOutTime{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO staff_out_times (user_id, work_date, out_at, notes, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, work_date) WHERE voided_at IS NULL DO UPDATE SET
			out_at = EXCLUDED.out_at,
			notes = EXCLUDED.notes,
			updated_by = EXCLUDED.updated_by,
			updated_at = CURRENT_TIMESTAMP
		RETURNING out_time_id, user_id, '' AS full_name, '' AS role, work_date, out_at, COALESCE(notes, ''), created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		outTime.UserID, outTime.WorkDate.Format("2006-01-02"), outTime.OutAt, outTime.Notes, outTime.CreatedBy, outTime.UpdatedBy,
	).Scan(staffOutTimeScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillStaffOutTimeDate(&out)
	return &out, nil
}

func (r *staffOutTimeRepoImpl) UpdateStaffOutTime(ctx context.Context, outTime model.StaffOutTime) (*model.StaffOutTime, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffOutTime{}
	err := r.db.QueryRow(ctx, `
		UPDATE staff_out_times SET
			user_id = $2,
			work_date = $3,
			out_at = $4,
			notes = $5,
			updated_by = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE out_time_id = $1 AND voided_at IS NULL
		RETURNING out_time_id, user_id, '' AS full_name, '' AS role, work_date, out_at, COALESCE(notes, ''), created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		outTime.OutTimeID, outTime.UserID, outTime.WorkDate.Format("2006-01-02"), outTime.OutAt, outTime.Notes, outTime.UpdatedBy,
	).Scan(staffOutTimeScanTargets(&out)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillStaffOutTimeDate(&out)
	return &out, nil
}

func (r *staffOutTimeRepoImpl) VoidStaffOutTime(ctx context.Context, outTimeID int64, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		UPDATE staff_out_times
		SET voided_at = CURRENT_TIMESTAMP, voided_by = $2, updated_by = $2, updated_at = CURRENT_TIMESTAMP
		WHERE out_time_id = $1 AND voided_at IS NULL`, outTimeID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

const staffOutTimeSelectSQL = `SELECT sot.out_time_id, sot.user_id, u.full_name, u.role, sot.work_date, sot.out_at, COALESCE(sot.notes, ''),
	sot.created_by, sot.updated_by, sot.voided_by, sot.voided_at, sot.created_at, sot.updated_at
	FROM staff_out_times sot
	JOIN users u ON u.user_id = sot.user_id`

func staffOutTimeScanTargets(outTime *model.StaffOutTime) []any {
	return []any{&outTime.OutTimeID, &outTime.UserID, &outTime.FullName, &outTime.Role, &outTime.WorkDate, &outTime.OutAt, &outTime.Notes, &outTime.CreatedBy, &outTime.UpdatedBy, &outTime.VoidedBy, &outTime.VoidedAt, &outTime.CreatedAt, &outTime.UpdatedAt}
}

func fillStaffOutTimeDate(outTime *model.StaffOutTime) {
	outTime.Date = outTime.WorkDate.Format("2006-01-02")
}
