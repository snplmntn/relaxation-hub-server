package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type StaffPayrollAdjustmentFilter struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	UserID      *int64
	Role        string
}

type PayrollAttendanceSource struct {
	AttendanceID               int64
	UserID                     int64
	Role                       model.PayrollRole
	FullName                   string
	UsualBranchIDSnapshot      *int64
	UsualLocationLabelSnapshot string
	WorkDate                   time.Time
	TimeInAt                   *time.Time
	TimeOutAt                  *time.Time
	SourceUpdatedAt            time.Time
}

type PayrollBookingSource struct {
	BookingID                  int64
	UserID                     int64
	Role                       model.PayrollRole
	FullName                   string
	UsualBranchIDSnapshot      *int64
	UsualLocationLabelSnapshot string
	BusinessDate               time.Time
	Status                     string
	ServiceName                string
	DurationMinutes            int
	FinalTotalCents            model.PayrollMoneyCents
	TherapistEarningsCents     model.PayrollMoneyCents
	SourceUpdatedAt            time.Time
}

type PayrollAdjustmentSource struct {
	AdjustmentID               int64
	UserID                     int64
	Role                       model.PayrollRole
	FullName                   string
	UsualBranchIDSnapshot      *int64
	UsualLocationLabelSnapshot string
	AdjustmentDate             time.Time
	Type                       model.PayrollAdjustmentType
	Category                   string
	AmountCents                model.PayrollMoneyCents
	Reason                     string
	SourceUpdatedAt            time.Time
}

type PayrollRepository interface {
	GetOpenCompensationRate(ctx context.Context, userID int64) (*model.StaffCompensationRate, error)
	CloseCompensationRate(ctx context.Context, rateID int64, effectiveTo time.Time, actorID int64) error
	CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error)
	CreateCompensationRateAtomic(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error)
	IsCompensationRateLocked(ctx context.Context, rateID int64) (bool, error)
	ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error)
	UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error
	ListStaffPayrollAdjustments(ctx context.Context, filter StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error)
	CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error)
	UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error)
	VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error
	IsStaffPayrollAdjustmentLocked(ctx context.Context, adjustmentID int64) (bool, error)
	ListPayrollAttendanceSources(ctx context.Context, start, end time.Time) ([]PayrollAttendanceSource, error)
	ListPayrollTherapistBookingSources(ctx context.Context, start, end time.Time) ([]PayrollBookingSource, error)
	ListPayrollAdjustmentSources(ctx context.Context, start, end time.Time) ([]PayrollAdjustmentSource, error)
	FindEffectiveRate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffCompensationRate, error)
	CreatePayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error)
	CreatePayrollRow(ctx context.Context, row model.PayrollRow) (*model.PayrollRow, error)
	CreatePayrollAttendanceDetail(ctx context.Context, rowID int64, detail model.PayrollAttendanceDetail) error
	CreatePayrollBookingDetail(ctx context.Context, rowID int64, detail model.PayrollBookingDetail) error
	CreatePayrollAdjustmentDetail(ctx context.Context, rowID int64, detail model.PayrollAdjustmentDetail) error
	VoidDraftRunsForPeriod(ctx context.Context, start, end time.Time, actorID int64, replacementRunID int64) error
	PersistDraftPayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error)
	HasActivePayrollCoverage(ctx context.Context, sourceKind string, sourceID int64) (bool, error)
	ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error)
	GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error)
	ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error
	VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error
	GetPayrollRunForUpdate(ctx context.Context, runID int64) (*model.PayrollRun, error)
	ListPayrollRows(ctx context.Context, runID int64) ([]model.PayrollRow, error)
	MarkPayrollRowPaid(ctx context.Context, rowID int64, paidBy int64, method, reference, notes string, ledgerEntryID int64) error
	RecordPayrollRowPayment(ctx context.Context, runID, rowID, paidBy int64, method, reference, notes string) (*model.PayrollRow, error)
	UpdatePayrollRunPaidIfComplete(ctx context.Context, runID int64) error
	CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error)
}

type payrollRepoImpl struct {
	db db.DBTX
}

type compensationRateRange struct {
	RateID        int64
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
}

const payrollCompensationRateAdvisoryNamespace int64 = 4477000000000000000

func NewPayrollRepository(database db.DBTX) PayrollRepository {
	return &payrollRepoImpl{db: database}
}

func (r *payrollRepoImpl) PayrollSettlementRecorder() PayrollSettlementRecorder {
	return NewLedgerRepository(r.db).(PayrollSettlementRecorder)
}

func payrollCompensationRateAdvisoryLockKey(userID int64) int64 {
	return payrollCompensationRateAdvisoryNamespace + userID
}

func payrollRunPeriodAdvisoryLockKey(start, end time.Time) int64 {
	return payrollDateKey(start)*100000000 + payrollDateKey(end)
}

func payrollDateKey(date time.Time) int64 {
	year, month, day := date.Date()
	return int64(year)*10000 + int64(month)*100 + int64(day)
}

func (r *payrollRepoImpl) GetOpenCompensationRate(ctx context.Context, userID int64) (*model.StaffCompensationRate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rate := model.StaffCompensationRate{}
	err := r.db.QueryRow(ctx, payrollRateSelectSQL+`
		WHERE user_id = $1 AND effective_to IS NULL
		ORDER BY effective_from DESC
		LIMIT 1`, userID).Scan(payrollRateScanTargets(&rate)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillPayrollRateDates(&rate)
	return &rate, nil
}

func (r *payrollRepoImpl) CloseCompensationRate(ctx context.Context, rateID int64, effectiveTo time.Time, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		UPDATE staff_compensation_rates
		SET effective_to = $2, updated_by = $3, updated_at = CURRENT_TIMESTAMP
		WHERE rate_id = $1
		  AND effective_to IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_rows pr
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pr.user_id = staff_compensation_rates.user_id
			  AND pr.role = staff_compensation_rates.role
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
			  AND run.period_start <= COALESCE(staff_compensation_rates.effective_to, 'infinity'::date)
			  AND run.period_end >= staff_compensation_rates.effective_from
		  )`, rateID, effectiveTo.Format("2006-01-02"), actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		locked, err := r.IsCompensationRateLocked(ctx, rateID)
		if err != nil {
			return err
		}
		if locked {
			return model.ErrPayrollRateLocked
		}
		return model.ErrNotFound
	}
	return nil
}

func (r *payrollRepoImpl) CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return insertCompensationRate(ctx, r.db, rate)
}

func (r *payrollRepoImpl) CreateCompensationRateAtomic(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := lockCompensationRateUser(ctx, tx, rate.UserID); err != nil {
		return nil, err
	}

	ranges, err := lockCompensationRateRanges(ctx, tx, rate.UserID)
	if err != nil {
		return nil, err
	}

	var openRange *compensationRateRange
	if rate.EffectiveTo == nil {
		for i := range ranges {
			if ranges[i].EffectiveTo == nil {
				if openRange != nil {
					return nil, model.ErrInvalidPayrollRate
				}
				openRange = &ranges[i]
			}
		}
	}

	if openRange != nil {
		locked, err := scanCompensationRateLocked(ctx, tx, openRange.RateID)
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, model.ErrPayrollRateLocked
		}
		closeTo := rate.EffectiveFrom.AddDate(0, 0, -1)
		if closeTo.Before(openRange.EffectiveFrom) {
			return nil, model.ErrInvalidPayrollRate
		}
		openRange.EffectiveTo = &closeTo
	}

	for _, existing := range ranges {
		if payrollDateRangesOverlap(rate.EffectiveFrom, rate.EffectiveTo, existing.EffectiveFrom, existing.EffectiveTo) {
			return nil, model.ErrInvalidPayrollRate
		}
	}

	if openRange != nil {
		var updatedBy interface{} = rate.UpdatedBy
		if rate.UpdatedBy != nil {
			updatedBy = *rate.UpdatedBy
		}
		tag, err := tx.Exec(ctx, `
			UPDATE staff_compensation_rates
			SET effective_to = $2, updated_by = $3, updated_at = CURRENT_TIMESTAMP
			WHERE rate_id = $1
			  AND effective_to IS NULL
			  AND NOT EXISTS (
				SELECT 1
				FROM payroll_rows pr
				JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
				WHERE pr.user_id = staff_compensation_rates.user_id
				  AND pr.role = staff_compensation_rates.role
				  AND run.status IN ('approved', 'paid')
				  AND run.voided_at IS NULL
				  AND pr.status <> 'voided'
				  AND run.period_start <= COALESCE(staff_compensation_rates.effective_to, 'infinity'::date)
				  AND run.period_end >= staff_compensation_rates.effective_from
			  )`, openRange.RateID, openRange.EffectiveTo.Format("2006-01-02"), updatedBy)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			locked, err := scanCompensationRateLocked(ctx, tx, openRange.RateID)
			if err != nil {
				return nil, err
			}
			if locked {
				return nil, model.ErrPayrollRateLocked
			}
			return nil, model.ErrInvalidPayrollRate
		}
	}

	out, err := insertCompensationRate(ctx, tx, rate)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func insertCompensationRate(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error) {

	out := model.StaffCompensationRate{}
	err := q.QueryRow(ctx, `
		INSERT INTO staff_compensation_rates (
			user_id, role, daily_rate_cents, overtime_multiplier, effective_from, effective_to,
			notes, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING rate_id, user_id, role, daily_rate_cents, overtime_multiplier, effective_from,
			effective_to, COALESCE(notes, ''), created_by, updated_by, created_at, updated_at`,
		rate.UserID, rate.Role, rate.DailyRateCents, rate.OvertimeMultiplier, rate.EffectiveFrom.Format("2006-01-02"),
		rate.EffectiveTo, rate.Notes, rate.CreatedBy,
	).Scan(payrollRateScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillPayrollRateDates(&out)
	return &out, nil
}

func lockCompensationRateUser(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}, userID int64) error {
	_, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, payrollCompensationRateAdvisoryLockKey(userID))
	return err
}

func lockCompensationRateRanges(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}, userID int64) ([]compensationRateRange, error) {
	rows, err := q.Query(ctx, `
		SELECT rate_id, effective_from, effective_to
		FROM staff_compensation_rates
		WHERE user_id = $1
		ORDER BY effective_from
		FOR UPDATE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ranges := make([]compensationRateRange, 0)
	for rows.Next() {
		var item compensationRateRange
		if err := rows.Scan(&item.RateID, &item.EffectiveFrom, &item.EffectiveTo); err != nil {
			return nil, err
		}
		ranges = append(ranges, item)
	}
	return ranges, rows.Err()
}

func payrollDateRangesOverlap(aStart time.Time, aEnd *time.Time, bStart time.Time, bEnd *time.Time) bool {
	if aEnd != nil && aEnd.Before(bStart) {
		return false
	}
	if bEnd != nil && bEnd.Before(aStart) {
		return false
	}
	return true
}

func (r *payrollRepoImpl) IsCompensationRateLocked(ctx context.Context, rateID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanCompensationRateLocked(ctx, r.db, rateID)
}

func scanCompensationRateLocked(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, rateID int64) (bool, error) {
	var locked bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM staff_compensation_rates scr
			JOIN payroll_rows pr ON pr.user_id = scr.user_id AND pr.role = scr.role
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE scr.rate_id = $1
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
			  AND run.period_start <= COALESCE(scr.effective_to, 'infinity'::date)
			  AND run.period_end >= scr.effective_from
		)`, rateID).Scan(&locked)
	if err != nil {
		return false, err
	}
	return locked, nil
}

func (r *payrollRepoImpl) ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, payrollRateSelectSQL+`
		WHERE ($1::bigint IS NULL OR user_id = $1)
		  AND ($2 = '' OR role = $2)
		ORDER BY user_id, effective_from DESC`, userID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]model.StaffCompensationRate, 0)
	for rows.Next() {
		rate := model.StaffCompensationRate{}
		if err := rows.Scan(payrollRateScanTargets(&rate)...); err != nil {
			return nil, err
		}
		fillPayrollRateDates(&rate)
		rates = append(rates, rate)
	}
	return rates, rows.Err()
}

func (r *payrollRepoImpl) UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		INSERT INTO staff_profiles (user_id, usual_branch_id, usual_location_label)
		SELECT $1, $2, $3
		FROM users
		WHERE user_id = $1
		  AND role = 'admin'
		  AND deleted_at IS NULL
		ON CONFLICT (user_id) DO UPDATE SET
			usual_branch_id = EXCLUDED.usual_branch_id,
			usual_location_label = EXCLUDED.usual_location_label,
			updated_at = CURRENT_TIMESTAMP`, userID, branchID, locationLabel)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidPayrollRole
	}
	return err
}

func (r *payrollRepoImpl) ListStaffPayrollAdjustments(ctx context.Context, filter StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, payrollAdjustmentSelectSQL+`
		WHERE spa.voided_at IS NULL
		  AND spa.period_start <= $2
		  AND spa.period_end >= $1
		  AND ($3::bigint IS NULL OR spa.user_id = $3)
		  AND ($4 = '' OR spa.role = $4)
		ORDER BY spa.adjustment_date DESC, spa.adjustment_id DESC`,
		filter.PeriodStart.Format("2006-01-02"), filter.PeriodEnd.Format("2006-01-02"), filter.UserID, filter.Role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.StaffPayrollAdjustment, 0)
	for rows.Next() {
		item := model.StaffPayrollAdjustment{}
		if err := rows.Scan(payrollAdjustmentScanTargets(&item)...); err != nil {
			return nil, err
		}
		fillPayrollAdjustmentDates(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffPayrollAdjustment{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO staff_payroll_adjustments (
			user_id, role, adjustment_date, period_start, period_end, type, category,
			amount_cents, reason, cash_movement_cents, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
		RETURNING adjustment_id, user_id, '' AS full_name, role, adjustment_date, period_start,
			period_end, type, category, amount_cents, reason, cash_movement_cents, created_at, updated_at`,
		adjustment.UserID, adjustment.Role, adjustment.AdjustmentDate.Format("2006-01-02"),
		adjustment.PeriodStart.Format("2006-01-02"), adjustment.PeriodEnd.Format("2006-01-02"),
		adjustment.Type, adjustment.Category, adjustment.AmountCents, adjustment.Reason,
		adjustment.CashMovementCents, adjustment.CreatedBy,
	).Scan(payrollAdjustmentScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillPayrollAdjustmentDates(&out)
	return &out, nil
}

func (r *payrollRepoImpl) UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.StaffPayrollAdjustment{}
	err := r.db.QueryRow(ctx, `
		UPDATE staff_payroll_adjustments SET
			user_id = $2,
			role = $3,
			adjustment_date = $4,
			period_start = $5,
			period_end = $6,
			type = $7,
			category = $8,
			amount_cents = $9,
			reason = $10,
			cash_movement_cents = $11,
			updated_by = $12,
			updated_at = CURRENT_TIMESTAMP
		WHERE adjustment_id = $1
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_adjustment_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.adjustment_id = staff_payroll_adjustments.adjustment_id
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		  )
		RETURNING adjustment_id, user_id, '' AS full_name, role, adjustment_date, period_start,
			period_end, type, category, amount_cents, reason, cash_movement_cents, created_at, updated_at`,
		adjustment.AdjustmentID, adjustment.UserID, adjustment.Role, adjustment.AdjustmentDate.Format("2006-01-02"),
		adjustment.PeriodStart.Format("2006-01-02"), adjustment.PeriodEnd.Format("2006-01-02"),
		adjustment.Type, adjustment.Category, adjustment.AmountCents, adjustment.Reason,
		adjustment.CashMovementCents, adjustment.UpdatedBy,
	).Scan(payrollAdjustmentScanTargets(&out)...)
	if errors.Is(err, pgx.ErrNoRows) {
		locked, lockErr := r.IsStaffPayrollAdjustmentLocked(ctx, adjustment.AdjustmentID)
		if lockErr != nil {
			return nil, lockErr
		}
		if locked {
			return nil, model.ErrPayrollAdjustmentLocked
		}
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillPayrollAdjustmentDates(&out)
	return &out, nil
}

func (r *payrollRepoImpl) VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		UPDATE staff_payroll_adjustments
		SET voided_at = CURRENT_TIMESTAMP, voided_by = $2, updated_by = $2, updated_at = CURRENT_TIMESTAMP
		WHERE adjustment_id = $1
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_adjustment_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.adjustment_id = staff_payroll_adjustments.adjustment_id
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		  )`, adjustmentID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		locked, err := r.IsStaffPayrollAdjustmentLocked(ctx, adjustmentID)
		if err != nil {
			return err
		}
		if locked {
			return model.ErrPayrollAdjustmentLocked
		}
		return model.ErrNotFound
	}
	return nil
}

func (r *payrollRepoImpl) IsStaffPayrollAdjustmentLocked(ctx context.Context, adjustmentID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var locked bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM payroll_adjustment_details pad
			JOIN payroll_rows pr ON pr.payroll_row_id = pad.payroll_row_id
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			WHERE pad.adjustment_id = $1
			  AND run.status IN ('approved', 'paid')
			  AND run.voided_at IS NULL
			  AND pr.status <> 'voided'
		)`, adjustmentID).Scan(&locked)
	if err != nil {
		return false, err
	}
	return locked, nil
}

func (r *payrollRepoImpl) ListPayrollAttendanceSources(ctx context.Context, start, end time.Time) ([]PayrollAttendanceSource, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT sae.attendance_id, sae.user_id, u.role, u.full_name,
		       CASE WHEN u.role = 'rider' THEN rp.usual_branch_id ELSE sp.usual_branch_id END AS usual_branch_id_snapshot,
		       COALESCE(CASE WHEN u.role = 'rider' THEN rp.usual_location_label ELSE sp.usual_location_label END, '') AS usual_location_label_snapshot,
		       sae.work_date, sae.time_in_at, sae.time_out_at, sae.updated_at
		FROM staff_attendance_entries sae
		JOIN users u ON u.user_id = sae.user_id
		LEFT JOIN rider_profiles rp ON rp.user_id = sae.user_id AND u.role = 'rider'
		LEFT JOIN staff_profiles sp ON sp.user_id = sae.user_id AND u.role = 'admin'
		WHERE sae.voided_at IS NULL
		  AND u.deleted_at IS NULL
		  AND sae.work_date BETWEEN $1 AND $2
		ORDER BY sae.user_id, sae.work_date, sae.attendance_id`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PayrollAttendanceSource, 0)
	for rows.Next() {
		var item PayrollAttendanceSource
		if err := rows.Scan(&item.AttendanceID, &item.UserID, &item.Role, &item.FullName, &item.UsualBranchIDSnapshot, &item.UsualLocationLabelSnapshot, &item.WorkDate, &item.TimeInAt, &item.TimeOutAt, &item.SourceUpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) ListPayrollTherapistBookingSources(ctx context.Context, start, end time.Time) ([]PayrollBookingSource, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT b.booking_id, b.therapist_id, u.role, u.full_name,
		       tp.branch_id AS usual_branch_id_snapshot,
		       COALESCE(br.name, '') AS usual_location_label_snapshot,
		       business_day(b.scheduled_start) AS business_date,
		       b.status, COALESCE(s.name, 'Service'), b.duration_minutes,
		       ROUND(COALESCE(b.final_total, 0) * 100)::bigint AS final_total_cents,
		       ROUND(COALESCE(b.therapist_earnings, 0) * 100)::bigint AS therapist_earnings_cents,
		       b.updated_at
		FROM bookings b
		JOIN users u ON u.user_id = b.therapist_id
		LEFT JOIN therapist_profiles tp ON tp.therapist_id = b.therapist_id
		LEFT JOIN branches br ON br.branch_id = tp.branch_id
		LEFT JOIN services s ON s.service_id = b.service_id
		WHERE b.actual_end IS NOT NULL
		  AND b.therapist_id IS NOT NULL
		  AND u.deleted_at IS NULL
		  AND business_day(b.scheduled_start) BETWEEN $1 AND $2
		ORDER BY b.therapist_id, business_date, b.booking_id`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PayrollBookingSource, 0)
	for rows.Next() {
		var item PayrollBookingSource
		if err := rows.Scan(&item.BookingID, &item.UserID, &item.Role, &item.FullName, &item.UsualBranchIDSnapshot, &item.UsualLocationLabelSnapshot, &item.BusinessDate, &item.Status, &item.ServiceName, &item.DurationMinutes, &item.FinalTotalCents, &item.TherapistEarningsCents, &item.SourceUpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) ListPayrollAdjustmentSources(ctx context.Context, start, end time.Time) ([]PayrollAdjustmentSource, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT spa.adjustment_id, spa.user_id, spa.role, u.full_name,
		       CASE
		         WHEN spa.role = 'rider' THEN rp.usual_branch_id
		         WHEN spa.role = 'admin' THEN sp.usual_branch_id
		         ELSE tp.branch_id
		       END AS usual_branch_id_snapshot,
		       COALESCE(CASE
		         WHEN spa.role = 'rider' THEN rp.usual_location_label
		         WHEN spa.role = 'admin' THEN sp.usual_location_label
		         ELSE br.name
		       END, '') AS usual_location_label_snapshot,
		       spa.adjustment_date, spa.type, spa.category, spa.amount_cents, spa.reason, spa.updated_at
		FROM staff_payroll_adjustments spa
		JOIN users u ON u.user_id = spa.user_id
		LEFT JOIN rider_profiles rp ON rp.user_id = spa.user_id AND spa.role = 'rider'
		LEFT JOIN staff_profiles sp ON sp.user_id = spa.user_id AND spa.role = 'admin'
		LEFT JOIN therapist_profiles tp ON tp.therapist_id = spa.user_id AND spa.role = 'therapist'
		LEFT JOIN branches br ON br.branch_id = tp.branch_id
		WHERE spa.voided_at IS NULL
		  AND spa.period_start <= $2
		  AND spa.period_end >= $1
		  AND u.deleted_at IS NULL
		ORDER BY spa.user_id, spa.adjustment_date, spa.adjustment_id`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PayrollAdjustmentSource, 0)
	for rows.Next() {
		var item PayrollAdjustmentSource
		if err := rows.Scan(&item.AdjustmentID, &item.UserID, &item.Role, &item.FullName, &item.UsualBranchIDSnapshot, &item.UsualLocationLabelSnapshot, &item.AdjustmentDate, &item.Type, &item.Category, &item.AmountCents, &item.Reason, &item.SourceUpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) FindEffectiveRate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffCompensationRate, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rate := model.StaffCompensationRate{}
	err := r.db.QueryRow(ctx, payrollRateSelectSQL+`
		WHERE user_id = $1
		  AND effective_from <= $2
		  AND (effective_to IS NULL OR effective_to >= $2)
		ORDER BY effective_from DESC
		LIMIT 1`, userID, workDate.Format("2006-01-02")).Scan(payrollRateScanTargets(&rate)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillPayrollRateDates(&rate)
	return &rate, nil
}

func (r *payrollRepoImpl) CreatePayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return insertPayrollRun(ctx, r.db, run)
}

func insertPayrollRun(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, run model.PayrollRun) (*model.PayrollRun, error) {
	out := model.PayrollRun{}
	err := q.QueryRow(ctx, `
		INSERT INTO payroll_runs (period_start, period_end, status, generated_by)
		VALUES ($1, $2, $3, $4)
		RETURNING payroll_run_id, period_start, period_end, status, generated_by, generated_at,
			approved_by, approved_at, voided_by, voided_at, COALESCE(voided_reason, '')`,
		run.PeriodStart.Format("2006-01-02"), run.PeriodEnd.Format("2006-01-02"), run.Status, run.GeneratedBy,
	).Scan(payrollRunScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillPayrollRunDates(&out)
	return &out, nil
}

func (r *payrollRepoImpl) CreatePayrollRow(ctx context.Context, row model.PayrollRow) (*model.PayrollRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return insertPayrollRow(ctx, r.db, row)
}

func insertPayrollRow(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, row model.PayrollRow) (*model.PayrollRow, error) {
	out := model.PayrollRow{}
	err := q.QueryRow(ctx, `
		INSERT INTO payroll_rows (
			payroll_run_id, user_id, role, full_name_snapshot, usual_branch_id_snapshot,
			usual_location_label_snapshot, status, regular_minutes, overtime_minutes,
			daily_rate_cents, overtime_multiplier, gross_cents, add_adjustments_cents,
			minus_adjustments_cents, final_pay_cents, blocker_codes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING payroll_row_id, payroll_run_id, user_id, role, full_name_snapshot,
			usual_branch_id_snapshot, COALESCE(usual_location_label_snapshot, ''), status,
			regular_minutes, overtime_minutes, daily_rate_cents, overtime_multiplier,
			gross_cents, add_adjustments_cents, minus_adjustments_cents, final_pay_cents,
			blocker_codes, paid_at, paid_by, COALESCE(payment_method, ''), COALESCE(payment_reference, ''),
			COALESCE(payment_notes, ''), ledger_entry_id, created_at, updated_at`,
		row.PayrollRunID, row.UserID, row.Role, row.FullNameSnapshot, row.UsualBranchIDSnapshot,
		row.UsualLocationLabelSnapshot, row.Status, row.RegularMinutes, row.OvertimeMinutes,
		row.DailyRateCents, row.OvertimeMultiplier, row.GrossCents, row.AddAdjustmentsCents,
		row.MinusAdjustmentsCents, row.FinalPayCents, row.BlockerCodes,
	).Scan(payrollRowScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *payrollRepoImpl) CreatePayrollAttendanceDetail(ctx context.Context, rowID int64, detail model.PayrollAttendanceDetail) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := insertPayrollAttendanceDetail(ctx, r.db, rowID, detail)
	return err
}

func insertPayrollAttendanceDetail(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, rowID int64, detail model.PayrollAttendanceDetail) (*model.PayrollAttendanceDetail, error) {
	out := model.PayrollAttendanceDetail{}
	err := q.QueryRow(ctx, `
		INSERT INTO payroll_attendance_details (
			payroll_row_id, attendance_id, work_date, time_in_at, time_out_at,
			worked_minutes, regular_minutes, overtime_minutes, daily_rate_cents,
			overtime_multiplier, gross_cents, source_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING detail_id, payroll_row_id, attendance_id, work_date, time_in_at, time_out_at,
			worked_minutes, regular_minutes, overtime_minutes, daily_rate_cents,
			overtime_multiplier, gross_cents, source_updated_at, created_at`,
		rowID, detail.AttendanceID, detail.WorkDate.Format("2006-01-02"), detail.TimeInAt, detail.TimeOutAt,
		detail.WorkedMinutes, detail.RegularMinutes, detail.OvertimeMinutes, detail.DailyRateCents,
		detail.OvertimeMultiplier, detail.GrossCents, detail.SourceUpdatedAt,
	).Scan(&out.DetailID, &out.PayrollRowID, &out.AttendanceID, &out.WorkDate, &out.TimeInAt, &out.TimeOutAt, &out.WorkedMinutes, &out.RegularMinutes, &out.OvertimeMinutes, &out.DailyRateCents, &out.OvertimeMultiplier, &out.GrossCents, &out.SourceUpdatedAt, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.Date = out.WorkDate.Format("2006-01-02")
	return &out, nil
}

func (r *payrollRepoImpl) CreatePayrollBookingDetail(ctx context.Context, rowID int64, detail model.PayrollBookingDetail) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := insertPayrollBookingDetail(ctx, r.db, rowID, detail)
	return err
}

func insertPayrollBookingDetail(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, rowID int64, detail model.PayrollBookingDetail) (*model.PayrollBookingDetail, error) {
	out := model.PayrollBookingDetail{}
	err := q.QueryRow(ctx, `
		INSERT INTO payroll_booking_details (
			payroll_row_id, booking_id, business_date, service_name, duration_minutes,
			final_total_cents, therapist_earnings_cents, source_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING detail_id, payroll_row_id, booking_id, business_date, service_name,
			duration_minutes, final_total_cents, therapist_earnings_cents, source_updated_at, created_at`,
		rowID, detail.BookingID, detail.BusinessDate.Format("2006-01-02"), detail.ServiceName,
		detail.DurationMinutes, detail.FinalTotalCents, detail.TherapistEarningsCents, detail.SourceUpdatedAt,
	).Scan(&out.DetailID, &out.PayrollRowID, &out.BookingID, &out.BusinessDate, &out.ServiceName, &out.DurationMinutes, &out.FinalTotalCents, &out.TherapistEarningsCents, &out.SourceUpdatedAt, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.Date = out.BusinessDate.Format("2006-01-02")
	return &out, nil
}

func (r *payrollRepoImpl) CreatePayrollAdjustmentDetail(ctx context.Context, rowID int64, detail model.PayrollAdjustmentDetail) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := insertPayrollAdjustmentDetail(ctx, r.db, rowID, detail)
	return err
}

func insertPayrollAdjustmentDetail(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, rowID int64, detail model.PayrollAdjustmentDetail) (*model.PayrollAdjustmentDetail, error) {
	out := model.PayrollAdjustmentDetail{}
	err := q.QueryRow(ctx, `
		INSERT INTO payroll_adjustment_details (
			payroll_row_id, adjustment_id, adjustment_date, type, category,
			amount_cents, reason, source_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING detail_id, payroll_row_id, adjustment_id, adjustment_date, type,
			category, amount_cents, reason, source_updated_at, created_at`,
		rowID, detail.AdjustmentID, detail.AdjustmentDate.Format("2006-01-02"), detail.Type,
		detail.Category, detail.AmountCents, detail.Reason, detail.SourceUpdatedAt,
	).Scan(&out.DetailID, &out.PayrollRowID, &out.AdjustmentID, &out.AdjustmentDate, &out.Type, &out.Category, &out.AmountCents, &out.Reason, &out.SourceUpdatedAt, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.Date = out.AdjustmentDate.Format("2006-01-02")
	return &out, nil
}

func (r *payrollRepoImpl) VoidDraftRunsForPeriod(ctx context.Context, start, end time.Time, actorID int64, replacementRunID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return voidDraftRunsForPeriod(ctx, r.db, start, end, actorID, replacementRunID)
}

func voidDraftRunsForPeriod(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}, start, end time.Time, actorID int64, replacementRunID int64) error {
	_, err := q.Exec(ctx, `
		UPDATE payroll_runs
		SET status = 'voided',
			voided_by = $3,
			voided_at = CURRENT_TIMESTAMP,
			voided_reason = 'replaced_by_new_draft',
			replaced_by_run_id = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE period_start = $1
		  AND period_end = $2
		  AND status = 'draft'
		  AND voided_at IS NULL
		  AND payroll_run_id <> $4`,
		start.Format("2006-01-02"), end.Format("2006-01-02"), actorID, replacementRunID)
	return err
}

func (r *payrollRepoImpl) PersistDraftPayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error) {
	if run.GeneratedBy == nil || *run.GeneratedBy <= 0 {
		return nil, model.ErrInvalidPayrollAdjustment
	}
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, payrollRunPeriodAdvisoryLockKey(run.PeriodStart, run.PeriodEnd)); err != nil {
		return nil, err
	}

	createdRun, err := insertPayrollRun(ctx, tx, run)
	if err != nil {
		return nil, err
	}
	createdRun.Rows = make([]model.PayrollRow, 0, len(run.Rows))
	for _, row := range run.Rows {
		row.PayrollRunID = createdRun.PayrollRunID
		attendanceDetails := row.AttendanceDetails
		bookingDetails := row.BookingDetails
		adjustmentDetails := row.AdjustmentDetails
		row.AttendanceDetails = nil
		row.BookingDetails = nil
		row.AdjustmentDetails = nil

		createdRow, err := insertPayrollRow(ctx, tx, row)
		if err != nil {
			return nil, err
		}
		for _, detail := range attendanceDetails {
			createdDetail, err := insertPayrollAttendanceDetail(ctx, tx, createdRow.PayrollRowID, detail)
			if err != nil {
				return nil, err
			}
			createdRow.AttendanceDetails = append(createdRow.AttendanceDetails, *createdDetail)
		}
		for _, detail := range bookingDetails {
			createdDetail, err := insertPayrollBookingDetail(ctx, tx, createdRow.PayrollRowID, detail)
			if err != nil {
				return nil, err
			}
			createdRow.BookingDetails = append(createdRow.BookingDetails, *createdDetail)
		}
		for _, detail := range adjustmentDetails {
			createdDetail, err := insertPayrollAdjustmentDetail(ctx, tx, createdRow.PayrollRowID, detail)
			if err != nil {
				return nil, err
			}
			createdRow.AdjustmentDetails = append(createdRow.AdjustmentDetails, *createdDetail)
		}
		createdRun.Rows = append(createdRun.Rows, *createdRow)
	}

	if err := voidDraftRunsForPeriod(ctx, tx, run.PeriodStart, run.PeriodEnd, *run.GeneratedBy, createdRun.PayrollRunID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return createdRun, nil
}

func (r *payrollRepoImpl) HasActivePayrollCoverage(ctx context.Context, sourceKind string, sourceID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var covered bool
	err := r.db.QueryRow(ctx, `
		SELECT CASE $1
			WHEN 'attendance' THEN EXISTS (
				SELECT 1
				FROM payroll_attendance_details d
				JOIN payroll_rows pr ON pr.payroll_row_id = d.payroll_row_id
				JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
				WHERE d.attendance_id = $2
				  AND run.status IN ('approved', 'paid')
				  AND run.voided_at IS NULL
				  AND pr.status <> 'voided'
			)
			WHEN 'booking' THEN EXISTS (
				SELECT 1
				FROM payroll_booking_details d
				JOIN payroll_rows pr ON pr.payroll_row_id = d.payroll_row_id
				JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
				WHERE d.booking_id = $2
				  AND run.status IN ('approved', 'paid')
				  AND run.voided_at IS NULL
				  AND pr.status <> 'voided'
			)
			WHEN 'adjustment' THEN EXISTS (
				SELECT 1
				FROM payroll_adjustment_details d
				JOIN payroll_rows pr ON pr.payroll_row_id = d.payroll_row_id
				JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
				WHERE d.adjustment_id = $2
				  AND run.status IN ('approved', 'paid')
				  AND run.voided_at IS NULL
				  AND pr.status <> 'voided'
			)
			ELSE false
		END`, sourceKind, sourceID).Scan(&covered)
	if err != nil {
		return false, err
	}
	return covered, nil
}

func (r *payrollRepoImpl) ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, payrollRunSelectSQL+` ORDER BY period_start DESC, payroll_run_id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]model.PayrollRun, 0)
	for rows.Next() {
		var run model.PayrollRun
		if err := rows.Scan(payrollRunScanTargets(&run)...); err != nil {
			return nil, err
		}
		fillPayrollRunDates(&run)
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *payrollRepoImpl) GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var run model.PayrollRun
	err := r.db.QueryRow(ctx, payrollRunSelectSQL+` WHERE payroll_run_id = $1`, runID).Scan(payrollRunScanTargets(&run)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillPayrollRunDates(&run)

	rows, err := r.listPayrollRows(ctx, run.PayrollRunID)
	if err != nil {
		return nil, err
	}
	run.Rows = rows
	return &run, nil
}

func (r *payrollRepoImpl) ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE payroll_runs
		SET status = 'approved',
			approved_by = $2,
			approved_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_run_id = $1
		  AND status = 'draft'
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_rows
			WHERE payroll_run_id = $1
			  AND (status = 'blocked' OR COALESCE(array_length(blocker_codes, 1), 0) > 0)
		  )`, runID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return r.payrollRunLifecycleError(ctx, tx, runID)
	}

	_, err = tx.Exec(ctx, `
		UPDATE payroll_rows
		SET status = 'approved',
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_run_id = $1
		  AND status = 'draft'
		  AND COALESCE(array_length(blocker_codes, 1), 0) = 0`, runID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *payrollRepoImpl) VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE payroll_runs
		SET status = 'voided',
			voided_by = $2,
			voided_at = CURRENT_TIMESTAMP,
			voided_reason = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_run_id = $1
		  AND status IN ('draft', 'approved')
		  AND voided_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_rows pr
			WHERE pr.payroll_run_id = payroll_runs.payroll_run_id
			  AND (pr.status = 'paid' OR pr.ledger_entry_id IS NOT NULL)
		  )`, runID, actorID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return r.payrollRunLifecycleError(ctx, tx, runID)
	}

	_, err = tx.Exec(ctx, `
		UPDATE payroll_rows
		SET status = 'voided',
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_run_id = $1
		  AND status IN ('draft', 'approved', 'blocked')`, runID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *payrollRepoImpl) GetPayrollRunForUpdate(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var run model.PayrollRun
	err := r.db.QueryRow(ctx, payrollRunSelectSQL+` WHERE payroll_run_id = $1 FOR UPDATE`, runID).Scan(payrollRunScanTargets(&run)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillPayrollRunDates(&run)
	return &run, nil
}

func (r *payrollRepoImpl) ListPayrollRows(ctx context.Context, runID int64) ([]model.PayrollRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.listPayrollRows(ctx, runID)
}

func (r *payrollRepoImpl) MarkPayrollRowPaid(ctx context.Context, rowID int64, paidBy int64, method, reference, notes string, ledgerEntryID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `
		UPDATE payroll_rows
		SET status = 'paid',
			paid_at = CURRENT_TIMESTAMP,
			paid_by = $2,
			payment_method = $3,
			payment_reference = $4,
			payment_notes = $5,
			ledger_entry_id = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_row_id = $1
		  AND status = 'approved'
		  AND ledger_entry_id IS NULL`, rowID, paidBy, method, reference, notes, ledgerEntryID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrPayrollRunImmutable
	}
	return nil
}

func (r *payrollRepoImpl) RecordPayrollRowPayment(ctx context.Context, runID, rowID, paidBy int64, method, reference, notes string) (*model.PayrollRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var runStatus model.PayrollRunStatus
	var periodStart time.Time
	var periodEnd time.Time
	var row model.PayrollRow
	err = tx.QueryRow(ctx, `
		SELECT run.status, run.period_start, run.period_end,
			pr.payroll_row_id, pr.payroll_run_id, pr.user_id, pr.role,
			pr.full_name_snapshot, pr.final_pay_cents, pr.ledger_entry_id, pr.status
		FROM payroll_runs run
		JOIN payroll_rows pr ON pr.payroll_run_id = run.payroll_run_id
		WHERE run.payroll_run_id = $1
		  AND pr.payroll_row_id = $2
		FOR UPDATE OF run, pr`, runID, rowID).Scan(
		&runStatus,
		&periodStart,
		&periodEnd,
		&row.PayrollRowID,
		&row.PayrollRunID,
		&row.UserID,
		&row.Role,
		&row.FullNameSnapshot,
		&row.FinalPayCents,
		&row.LedgerEntryID,
		&row.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if runStatus != model.PayrollRunStatusApproved || row.Status != model.PayrollRowStatusApproved || row.LedgerEntryID != nil {
		return nil, model.ErrPayrollRunImmutable
	}

	role, err := payrollLedgerTargetRole(row.Role)
	if err != nil {
		return nil, err
	}
	amount := float64(row.FinalPayCents) / 100.0
	var ledgerEntryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ledger_entries (
			entry_type, category, amount, description, entry_date, created_by,
			target_user_id, target_role, payroll_run_id, payroll_row_id, status
		)
		VALUES (
			'debit',
			'settlement',
			$5,
			CONCAT('Payroll settlement ', $9, ' to ', $10, ' - ', $11, ' via ', $6, CASE WHEN $7 = '' THEN '' ELSE CONCAT(' ref ', $7) END),
			NOW(),
			$8,
			$3,
			$4,
			$1,
			$2,
			'approved'
		)
		RETURNING entry_id
	`, runID, rowID, row.UserID, string(role), amount, method, reference, paidBy, periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02"), row.FullNameSnapshot).Scan(&ledgerEntryID)
	if err != nil {
		return nil, err
	}

	updated := model.PayrollRow{}
	err = tx.QueryRow(ctx, `
		UPDATE payroll_rows
		SET status = 'paid',
			paid_at = CURRENT_TIMESTAMP,
			paid_by = $2,
			payment_method = $3,
			payment_reference = $4,
			payment_notes = $5,
			ledger_entry_id = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_row_id = $1
		  AND status = 'approved'
		  AND ledger_entry_id IS NULL
		RETURNING payroll_row_id, payroll_run_id, user_id, role, full_name_snapshot,
			usual_branch_id_snapshot, COALESCE(usual_location_label_snapshot, ''), status,
			regular_minutes, overtime_minutes, daily_rate_cents, overtime_multiplier,
			gross_cents, add_adjustments_cents, minus_adjustments_cents, final_pay_cents,
			blocker_codes, paid_at, paid_by, COALESCE(payment_method, ''), COALESCE(payment_reference, ''),
			COALESCE(payment_notes, ''), ledger_entry_id, created_at, updated_at`, rowID, paidBy, method, reference, notes, ledgerEntryID).Scan(payrollRowScanTargets(&updated)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrPayrollRunImmutable
	}
	if err != nil {
		return nil, err
	}

	if err := updatePayrollRunPaidIfComplete(ctx, tx, runID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *payrollRepoImpl) UpdatePayrollRunPaidIfComplete(ctx context.Context, runID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return updatePayrollRunPaidIfComplete(ctx, r.db, runID)
}

func updatePayrollRunPaidIfComplete(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}, runID int64) error {
	_, err := q.Exec(ctx, `
		UPDATE payroll_runs
		SET status = 'paid',
			updated_at = CURRENT_TIMESTAMP
		WHERE payroll_run_id = $1
		  AND status = 'approved'
		  AND EXISTS (
			SELECT 1 FROM payroll_rows WHERE payroll_run_id = $1
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM payroll_rows
			WHERE payroll_run_id = $1
			  AND status <> 'paid'
		  )`, runID)
	return err
}

func (r *payrollRepoImpl) CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT reason
		FROM (
			SELECT 'attendance_source_updated' AS reason
			FROM payroll_rows pr
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			JOIN payroll_attendance_details pad ON pad.payroll_row_id = pr.payroll_row_id
			JOIN staff_attendance_entries sae ON sae.attendance_id = pad.attendance_id
			WHERE pr.payroll_run_id = $1
			  AND pr.status <> 'voided'
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND sae.updated_at > pad.source_updated_at

			UNION ALL

			SELECT 'booking_source_updated' AS reason
			FROM payroll_rows pr
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			JOIN payroll_booking_details pbd ON pbd.payroll_row_id = pr.payroll_row_id
			JOIN bookings b ON b.booking_id = pbd.booking_id
			WHERE pr.payroll_run_id = $1
			  AND pr.status <> 'voided'
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND b.updated_at > pbd.source_updated_at

			UNION ALL

			SELECT 'adjustment_source_updated' AS reason
			FROM payroll_rows pr
			JOIN payroll_runs run ON run.payroll_run_id = pr.payroll_run_id
			JOIN payroll_adjustment_details pad ON pad.payroll_row_id = pr.payroll_row_id
			JOIN staff_payroll_adjustments spa ON spa.adjustment_id = pad.adjustment_id
			WHERE pr.payroll_run_id = $1
			  AND pr.status <> 'voided'
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND spa.updated_at > pad.source_updated_at

			UNION ALL

			SELECT 'new_attendance_source' AS reason
			FROM payroll_runs run
			JOIN staff_attendance_entries sae
				ON sae.work_date BETWEEN run.period_start AND run.period_end
			JOIN users u ON u.user_id = sae.user_id
			WHERE run.payroll_run_id = $1
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND sae.voided_at IS NULL
			  AND u.deleted_at IS NULL
			  AND u.role IN ('rider', 'admin')
			  AND NOT EXISTS (
				SELECT 1
				FROM payroll_attendance_details existing
				JOIN payroll_rows existing_row ON existing_row.payroll_row_id = existing.payroll_row_id
				WHERE existing.attendance_id = sae.attendance_id
				  AND existing_row.payroll_run_id = run.payroll_run_id
				  AND existing_row.status <> 'voided'
			  )

			UNION ALL

			SELECT 'new_booking_source' AS reason
			FROM payroll_runs run
			JOIN bookings b
				ON business_day(b.scheduled_start) BETWEEN run.period_start AND run.period_end
			JOIN users u ON u.user_id = b.therapist_id
			WHERE run.payroll_run_id = $1
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND b.actual_end IS NOT NULL
			  AND b.therapist_id IS NOT NULL
			  AND b.status = 'completed'
			  AND COALESCE(b.therapist_earnings, 0) > 0
			  AND u.deleted_at IS NULL
			  AND NOT EXISTS (
				SELECT 1
				FROM payroll_booking_details existing
				JOIN payroll_rows existing_row ON existing_row.payroll_row_id = existing.payroll_row_id
				WHERE existing.booking_id = b.booking_id
				  AND existing_row.payroll_run_id = run.payroll_run_id
				  AND existing_row.status <> 'voided'
			  )

			UNION ALL

			SELECT 'new_adjustment_source' AS reason
			FROM payroll_runs run
			JOIN staff_payroll_adjustments spa
				ON spa.period_start <= run.period_end
				AND spa.period_end >= run.period_start
			JOIN users u ON u.user_id = spa.user_id
			WHERE run.payroll_run_id = $1
			  AND run.status <> 'voided'
			  AND run.voided_at IS NULL
			  AND spa.voided_at IS NULL
			  AND u.deleted_at IS NULL
			  AND NOT EXISTS (
				SELECT 1
				FROM payroll_adjustment_details existing
				JOIN payroll_rows existing_row ON existing_row.payroll_row_id = existing.payroll_row_id
				WHERE existing.adjustment_id = spa.adjustment_id
				  AND existing_row.payroll_run_id = run.payroll_run_id
				  AND existing_row.status <> 'voided'
			  )
		) stale_sources
		ORDER BY reason`, runID)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	reasons := make([]string, 0)
	for rows.Next() {
		var reason string
		if err := rows.Scan(&reason); err != nil {
			return false, nil, err
		}
		reasons = append(reasons, reason)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}
	return len(reasons) > 0, reasons, nil
}

func (r *payrollRepoImpl) payrollRunLifecycleError(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, runID int64) error {
	var status model.PayrollRunStatus
	var hasBlockers bool
	var hasPaidRows bool
	err := q.QueryRow(ctx, `
		SELECT run.status,
			EXISTS (
				SELECT 1
				FROM payroll_rows pr
				WHERE pr.payroll_run_id = run.payroll_run_id
				  AND (pr.status = 'blocked' OR COALESCE(array_length(pr.blocker_codes, 1), 0) > 0)
			),
			EXISTS (
				SELECT 1
				FROM payroll_rows pr
				WHERE pr.payroll_run_id = run.payroll_run_id
				  AND (pr.status = 'paid' OR pr.ledger_entry_id IS NOT NULL)
			)
		FROM payroll_runs run
		WHERE run.payroll_run_id = $1`, runID).Scan(&status, &hasBlockers, &hasPaidRows)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return err
	}
	if hasPaidRows {
		return model.ErrPayrollRunImmutable
	}
	if hasBlockers {
		return model.ErrPayrollRunHasBlockers
	}
	return model.ErrPayrollRunImmutable
}

func payrollLedgerTargetRole(role model.PayrollRole) (TargetRole, error) {
	switch string(role) {
	case model.RoleTherapist:
		return TargetRoleTherapist, nil
	case model.RoleRider:
		return TargetRoleRider, nil
	case model.RoleAdmin:
		return TargetRoleAdmin, nil
	default:
		return "", model.ErrInvalidPayrollRole
	}
}

func (r *payrollRepoImpl) listPayrollRows(ctx context.Context, runID int64) ([]model.PayrollRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT payroll_row_id, payroll_run_id, user_id, role, full_name_snapshot,
			usual_branch_id_snapshot, COALESCE(usual_location_label_snapshot, ''), status,
			regular_minutes, overtime_minutes, daily_rate_cents, overtime_multiplier,
			gross_cents, add_adjustments_cents, minus_adjustments_cents, final_pay_cents,
			blocker_codes, paid_at, paid_by, COALESCE(payment_method, ''), COALESCE(payment_reference, ''),
			COALESCE(payment_notes, ''), ledger_entry_id, created_at, updated_at
		FROM payroll_rows
		WHERE payroll_run_id = $1
		ORDER BY full_name_snapshot, payroll_row_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PayrollRow, 0)
	for rows.Next() {
		var item model.PayrollRow
		if err := rows.Scan(payrollRowScanTargets(&item)...); err != nil {
			return nil, err
		}
		if err := r.fillPayrollRowDetails(ctx, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) fillPayrollRowDetails(ctx context.Context, row *model.PayrollRow) error {
	attendance, err := r.listPayrollAttendanceDetails(ctx, row.PayrollRowID)
	if err != nil {
		return err
	}
	bookings, err := r.listPayrollBookingDetails(ctx, row.PayrollRowID)
	if err != nil {
		return err
	}
	adjustments, err := r.listPayrollAdjustmentDetails(ctx, row.PayrollRowID)
	if err != nil {
		return err
	}
	row.AttendanceDetails = attendance
	row.BookingDetails = bookings
	row.AdjustmentDetails = adjustments
	return nil
}

func (r *payrollRepoImpl) listPayrollAttendanceDetails(ctx context.Context, rowID int64) ([]model.PayrollAttendanceDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT detail_id, payroll_row_id, attendance_id, work_date, time_in_at, time_out_at,
			worked_minutes, regular_minutes, overtime_minutes, daily_rate_cents,
			overtime_multiplier, gross_cents, source_updated_at, created_at
		FROM payroll_attendance_details
		WHERE payroll_row_id = $1
		ORDER BY work_date, attendance_id`, rowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PayrollAttendanceDetail, 0)
	for rows.Next() {
		var item model.PayrollAttendanceDetail
		if err := rows.Scan(&item.DetailID, &item.PayrollRowID, &item.AttendanceID, &item.WorkDate, &item.TimeInAt, &item.TimeOutAt, &item.WorkedMinutes, &item.RegularMinutes, &item.OvertimeMinutes, &item.DailyRateCents, &item.OvertimeMultiplier, &item.GrossCents, &item.SourceUpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Date = item.WorkDate.Format("2006-01-02")
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) listPayrollBookingDetails(ctx context.Context, rowID int64) ([]model.PayrollBookingDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT detail_id, payroll_row_id, booking_id, business_date, service_name,
			duration_minutes, final_total_cents, therapist_earnings_cents, source_updated_at, created_at
		FROM payroll_booking_details
		WHERE payroll_row_id = $1
		ORDER BY business_date, booking_id`, rowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PayrollBookingDetail, 0)
	for rows.Next() {
		var item model.PayrollBookingDetail
		if err := rows.Scan(&item.DetailID, &item.PayrollRowID, &item.BookingID, &item.BusinessDate, &item.ServiceName, &item.DurationMinutes, &item.FinalTotalCents, &item.TherapistEarningsCents, &item.SourceUpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Date = item.BusinessDate.Format("2006-01-02")
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *payrollRepoImpl) listPayrollAdjustmentDetails(ctx context.Context, rowID int64) ([]model.PayrollAdjustmentDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT detail_id, payroll_row_id, adjustment_id, adjustment_date, type,
			category, amount_cents, reason, source_updated_at, created_at
		FROM payroll_adjustment_details
		WHERE payroll_row_id = $1
		ORDER BY adjustment_date, adjustment_id`, rowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PayrollAdjustmentDetail, 0)
	for rows.Next() {
		var item model.PayrollAdjustmentDetail
		if err := rows.Scan(&item.DetailID, &item.PayrollRowID, &item.AdjustmentID, &item.AdjustmentDate, &item.Type, &item.Category, &item.AmountCents, &item.Reason, &item.SourceUpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Date = item.AdjustmentDate.Format("2006-01-02")
		items = append(items, item)
	}
	return items, rows.Err()
}

const payrollRateSelectSQL = `SELECT rate_id, user_id, role, daily_rate_cents, overtime_multiplier, effective_from,
	effective_to, COALESCE(notes, ''), created_by, updated_by, created_at, updated_at
	FROM staff_compensation_rates`

func payrollRateScanTargets(rate *model.StaffCompensationRate) []any {
	return []any{&rate.RateID, &rate.UserID, &rate.Role, &rate.DailyRateCents, &rate.OvertimeMultiplier, &rate.EffectiveFrom, &rate.EffectiveTo, &rate.Notes, &rate.CreatedBy, &rate.UpdatedBy, &rate.CreatedAt, &rate.UpdatedAt}
}

func fillPayrollRateDates(rate *model.StaffCompensationRate) {
	rate.EffectiveFromDate = rate.EffectiveFrom.Format("2006-01-02")
	if rate.EffectiveTo != nil {
		value := rate.EffectiveTo.Format("2006-01-02")
		rate.EffectiveToDate = &value
	}
}

const payrollAdjustmentSelectSQL = `SELECT spa.adjustment_id, spa.user_id, u.full_name, spa.role, spa.adjustment_date,
	spa.period_start, spa.period_end, spa.type, spa.category, spa.amount_cents, spa.reason, spa.cash_movement_cents,
	spa.created_at, spa.updated_at
	FROM staff_payroll_adjustments spa
	JOIN users u ON u.user_id = spa.user_id`

func payrollAdjustmentScanTargets(adjustment *model.StaffPayrollAdjustment) []any {
	return []any{&adjustment.AdjustmentID, &adjustment.UserID, &adjustment.FullName, &adjustment.Role, &adjustment.AdjustmentDate, &adjustment.PeriodStart, &adjustment.PeriodEnd, &adjustment.Type, &adjustment.Category, &adjustment.AmountCents, &adjustment.Reason, &adjustment.CashMovementCents, &adjustment.CreatedAt, &adjustment.UpdatedAt}
}

func fillPayrollAdjustmentDates(adjustment *model.StaffPayrollAdjustment) {
	adjustment.Date = adjustment.AdjustmentDate.Format("2006-01-02")
	adjustment.PeriodStartDate = adjustment.PeriodStart.Format("2006-01-02")
	adjustment.PeriodEndDate = adjustment.PeriodEnd.Format("2006-01-02")
}

const payrollRunSelectSQL = `SELECT payroll_run_id, period_start, period_end, status, generated_by, generated_at,
	approved_by, approved_at, voided_by, voided_at, COALESCE(voided_reason, '')
	FROM payroll_runs`

func payrollRunScanTargets(run *model.PayrollRun) []any {
	return []any{&run.PayrollRunID, &run.PeriodStart, &run.PeriodEnd, &run.Status, &run.GeneratedBy, &run.GeneratedAt, &run.ApprovedBy, &run.ApprovedAt, &run.VoidedBy, &run.VoidedAt, &run.VoidedReason}
}

func fillPayrollRunDates(run *model.PayrollRun) {
	run.StartDate = run.PeriodStart.Format("2006-01-02")
	run.EndDate = run.PeriodEnd.Format("2006-01-02")
}

func payrollRowScanTargets(row *model.PayrollRow) []any {
	return []any{
		&row.PayrollRowID,
		&row.PayrollRunID,
		&row.UserID,
		&row.Role,
		&row.FullNameSnapshot,
		&row.UsualBranchIDSnapshot,
		&row.UsualLocationLabelSnapshot,
		&row.Status,
		&row.RegularMinutes,
		&row.OvertimeMinutes,
		&row.DailyRateCents,
		&row.OvertimeMultiplier,
		&row.GrossCents,
		&row.AddAdjustmentsCents,
		&row.MinusAdjustmentsCents,
		&row.FinalPayCents,
		&row.BlockerCodes,
		&row.PaidAt,
		&row.PaidBy,
		&row.PaymentMethod,
		&row.PaymentReference,
		&row.PaymentNotes,
		&row.LedgerEntryID,
		&row.CreatedAt,
		&row.UpdatedAt,
	}
}
