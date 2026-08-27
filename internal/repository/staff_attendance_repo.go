package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type StaffAttendanceRepository interface {
	ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error)
	ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error)
	GetStaffAttendanceTargetUser(ctx context.Context, userID int64) (*model.StaffAttendanceUser, error)
	GetStaffAttendance(ctx context.Context, attendanceID int64) (*model.StaffAttendance, error)
	GetActiveStaffAttendanceByUserDate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffAttendance, error)
	CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error)
	UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error)
	VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64) error
	IsAttendanceLocked(ctx context.Context, attendanceID int64) (bool, error)
}

type staffAttendanceRepoImpl struct {
	db db.DBTX
}

func NewStaffAttendanceRepository(database db.DBTX) StaffAttendanceRepository {
	return &staffAttendanceRepoImpl{db: database}
}

func (r *staffAttendanceRepoImpl) ListStaffAttendance(ctx context.Context, filter model.StaffAttendanceFilter) ([]model.StaffAttendance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, staffAttendanceSelectSQL+`
		WHERE sae.voided_at IS NULL
		  AND sae.work_date = $1
		  AND u.deleted_at IS NULL
		  AND u.role IN ('therapist', 'rider', 'admin')
		  AND ($2 = '' OR u.role = $2)
		  AND ($3 = '' OR u.full_name ILIKE '%' || $3 || '%' OR u.primary_email ILIKE '%' || $3 || '%' OR u.primary_phone ILIKE '%' || $3 || '%')
		ORDER BY u.role, u.full_name, sae.attendance_id`,
		filter.WorkDate.Format("2006-01-02"), strings.TrimSpace(filter.Role), strings.TrimSpace(filter.Search))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.StaffAttendance, 0)
	for rows.Next() {
		var item model.StaffAttendance
		if err := rows.Scan(staffAttendanceScanTargets(&item)...); err != nil {
			return nil, err
		}
		fillStaffAttendanceDate(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *staffAttendanceRepoImpl) ListStaffAttendanceAdminTargets(ctx context.Context, search string, limit int) ([]model.StaffAttendanceUser, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 100
	}
	search = strings.TrimSpace(search)

	rows, err := r.db.Query(ctx, `
		SELECT user_id, full_name, role
		FROM users
		WHERE deleted_at IS NULL
		  AND role = 'admin'
		  AND ($1 = '' OR full_name ILIKE '%' || $1 || '%' OR primary_email ILIKE '%' || $1 || '%' OR primary_phone ILIKE '%' || $1 || '%')
		ORDER BY full_name, user_id
		LIMIT $2`, search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.StaffAttendanceUser, 0)
	for rows.Next() {
		var item model.StaffAttendanceUser
		if err := rows.Scan(&item.UserID, &item.FullName, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *staffAttendanceRepoImpl) GetStaffAttendanceTargetUser(ctx context.Context, userID int64) (*model.StaffAttendanceUser, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	user := model.StaffAttendanceUser{}
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

func (r *staffAttendanceRepoImpl) GetStaffAttendance(ctx context.Context, attendanceID int64) (*model.StaffAttendance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	item := model.StaffAttendance{}
	err := r.db.QueryRow(ctx, staffAttendanceSelectSQL+`
		WHERE sae.attendance_id = $1
		  AND sae.voided_at IS NULL
		  AND u.deleted_at IS NULL`, attendanceID).Scan(staffAttendanceScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillStaffAttendanceDate(&item)
	return &item, nil
}

func (r *staffAttendanceRepoImpl) GetActiveStaffAttendanceByUserDate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffAttendance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	item := model.StaffAttendance{}
	err := r.db.QueryRow(ctx, staffAttendanceSelectSQL+`
		WHERE sae.user_id = $1
		  AND sae.work_date = $2
		  AND sae.voided_at IS NULL
		  AND u.deleted_at IS NULL`,
		userID, workDate.Format("2006-01-02")).Scan(staffAttendanceScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillStaffAttendanceDate(&item)
	return &item, nil
}

func (r *staffAttendanceRepoImpl) CreateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffAttendance{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO staff_attendance_entries (user_id, work_date, time_in_at, time_out_at, notes, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, work_date) WHERE voided_at IS NULL DO UPDATE SET
			time_in_at = EXCLUDED.time_in_at,
			time_out_at = EXCLUDED.time_out_at,
			notes = EXCLUDED.notes,
			updated_by = EXCLUDED.updated_by,
			updated_at = CURRENT_TIMESTAMP
		WHERE NOT EXISTS (
			SELECT 1
			FROM payroll_attendance_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.attendance_id = staff_attendance_entries.attendance_id
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		)
		RETURNING attendance_id, user_id, '' AS full_name, '' AS role, work_date, time_in_at, time_out_at, COALESCE(notes, ''),
			created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		attendance.UserID, attendance.WorkDate.Format("2006-01-02"), attendance.TimeInAt, attendance.TimeOutAt, attendance.Notes, attendance.CreatedBy, attendance.UpdatedBy,
	).Scan(staffAttendanceScanTargets(&out)...)
	if isStaffAttendanceDuplicateErr(err) {
		return nil, model.ErrStaffAttendanceDuplicate
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrStaffAttendanceLocked
	}
	if err != nil {
		return nil, err
	}
	fillStaffAttendanceDate(&out)
	return &out, nil
}

func (r *staffAttendanceRepoImpl) UpdateStaffAttendance(ctx context.Context, attendance model.StaffAttendance) (*model.StaffAttendance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffAttendance{}
	err := r.db.QueryRow(ctx, `
		UPDATE staff_attendance_entries SET
			user_id = $2,
			work_date = $3,
			time_in_at = $4,
			time_out_at = $5,
			notes = $6,
			updated_by = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE attendance_id = $1
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_attendance_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.attendance_id = staff_attendance_entries.attendance_id
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		  )
		RETURNING attendance_id, user_id, '' AS full_name, '' AS role, work_date, time_in_at, time_out_at, COALESCE(notes, ''),
			created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		attendance.AttendanceID, attendance.UserID, attendance.WorkDate.Format("2006-01-02"), attendance.TimeInAt, attendance.TimeOutAt, attendance.Notes, attendance.UpdatedBy,
	).Scan(staffAttendanceScanTargets(&out)...)
	if isStaffAttendanceDuplicateErr(err) {
		return nil, model.ErrStaffAttendanceDuplicate
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrStaffAttendanceLocked
	}
	if err != nil {
		return nil, err
	}
	fillStaffAttendanceDate(&out)
	return &out, nil
}

func (r *staffAttendanceRepoImpl) VoidStaffAttendance(ctx context.Context, attendanceID int64, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		UPDATE staff_attendance_entries
		SET voided_at = CURRENT_TIMESTAMP, voided_by = $2, updated_by = $2, updated_at = CURRENT_TIMESTAMP
		WHERE attendance_id = $1
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_attendance_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.attendance_id = staff_attendance_entries.attendance_id
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		  )`, attendanceID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		locked, err := r.IsAttendanceLocked(ctx, attendanceID)
		if err != nil {
			return err
		}
		if locked {
			return model.ErrStaffAttendanceLocked
		}
		return model.ErrNotFound
	}
	return nil
}

func (r *staffAttendanceRepoImpl) IsAttendanceLocked(ctx context.Context, attendanceID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var locked bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM payroll_attendance_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.attendance_id = $1
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		)`, attendanceID).Scan(&locked)
	if err != nil {
		return false, err
	}
	return locked, nil
}

const staffAttendanceSelectSQL = `SELECT sae.attendance_id, sae.user_id, u.full_name, u.role, sae.work_date, sae.time_in_at, sae.time_out_at, COALESCE(sae.notes, ''),
	sae.created_by, sae.updated_by, sae.voided_by, sae.voided_at, sae.created_at, sae.updated_at
	FROM staff_attendance_entries sae
	JOIN users u ON u.user_id = sae.user_id`

func staffAttendanceScanTargets(attendance *model.StaffAttendance) []any {
	return []any{&attendance.AttendanceID, &attendance.UserID, &attendance.FullName, &attendance.Role, &attendance.WorkDate, &attendance.TimeInAt, &attendance.TimeOutAt, &attendance.Notes, &attendance.CreatedBy, &attendance.UpdatedBy, &attendance.VoidedBy, &attendance.VoidedAt, &attendance.CreatedAt, &attendance.UpdatedAt}
}

func fillStaffAttendanceDate(attendance *model.StaffAttendance) {
	attendance.Date = attendance.WorkDate.Format("2006-01-02")
}

func isStaffAttendanceDuplicateErr(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "idx_staff_attendance_active_user_date"
}
