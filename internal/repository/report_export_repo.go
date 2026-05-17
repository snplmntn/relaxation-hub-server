package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type ReportExportRepository interface {
	ListActiveBranchTherapists(ctx context.Context) ([]model.ReportTherapistRosterRow, error)
	ListDailySalesBookingRows(ctx context.Context, businessDate time.Time) ([]model.ReportDailySalesBookingRow, error)
	CountDailySalesCompletedBookingsMissingActualEnd(ctx context.Context, businessDate time.Time) (int, error)
	CountSalaryCompletedBookingsMissingActualEnd(ctx context.Context, startDate time.Time, endDate time.Time) (int, error)
	GetDailySalesRemittance(ctx context.Context, businessDate time.Time, branchID int64) (*model.DailySalesRemittance, error)
	UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error)
	ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error)
	CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error
	ListSalaryBookingRows(ctx context.Context, filter model.SalaryReportFilter) ([]model.ReportSalaryBookingRow, error)
}

type reportExportRepoImpl struct {
	db db.DBTX
}

func NewReportExportRepository(db db.DBTX) ReportExportRepository {
	return &reportExportRepoImpl{db: db}
}

func (r *reportExportRepoImpl) ListActiveBranchTherapists(ctx context.Context) ([]model.ReportTherapistRosterRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT b.branch_id, b.branch_name, u.user_id, u.full_name
		FROM therapist_profiles tp
		JOIN users u ON u.user_id = tp.therapist_id
		JOIN branches b ON b.branch_id = tp.branch_id
		WHERE tp.deleted_at IS NULL
		  AND u.deleted_at IS NULL
		  AND u.account_status = 'active'
		  AND u.role = 'therapist'
		  AND b.deleted_at IS NULL
		  AND COALESCE(b.is_active, TRUE) = TRUE
		ORDER BY b.branch_name, u.full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ReportTherapistRosterRow, 0)
	for rows.Next() {
		var item model.ReportTherapistRosterRow
		if err := rows.Scan(&item.BranchID, &item.BranchName, &item.TherapistID, &item.TherapistName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *reportExportRepoImpl) ListDailySalesBookingRows(ctx context.Context, businessDate time.Time) ([]model.ReportDailySalesBookingRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(tp.branch_id, 0),
		       COALESCE(br.branch_name, 'Unassigned'),
		       b.therapist_id,
		       COALESCE(u.full_name, 'Unknown Therapist'),
		       COALESCE(b.payment_method, ''),
		       COALESCE(SUM(b.final_total), 0),
		       COALESCE(SUM(b.duration_minutes), 0)::float / 60.0,
		       COUNT(*)
		FROM bookings b
		JOIN users u ON u.user_id = b.therapist_id
		LEFT JOIN therapist_profiles tp ON tp.therapist_id = b.therapist_id
		LEFT JOIN branches br ON br.branch_id = tp.branch_id AND br.deleted_at IS NULL
		WHERE b.status = 'completed'
		  AND b.actual_end IS NOT NULL
		  AND b.therapist_id IS NOT NULL
		  AND DATE((b.actual_end AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Manila') = $1
		GROUP BY COALESCE(tp.branch_id, 0), COALESCE(br.branch_name, 'Unassigned'), b.therapist_id, COALESCE(u.full_name, 'Unknown Therapist'), COALESCE(b.payment_method, '')
		ORDER BY COALESCE(br.branch_name, 'Unassigned'), COALESCE(u.full_name, 'Unknown Therapist')`, businessDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ReportDailySalesBookingRow, 0)
	for rows.Next() {
		var item model.ReportDailySalesBookingRow
		if err := rows.Scan(&item.BranchID, &item.BranchName, &item.TherapistID, &item.TherapistName, &item.PaymentMethod, &item.TotalSales, &item.TotalHours, &item.BookingCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *reportExportRepoImpl) CountDailySalesCompletedBookingsMissingActualEnd(ctx context.Context, businessDate time.Time) (int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE status = 'completed'
		  AND actual_end IS NULL
		  AND DATE((COALESCE(actual_start, scheduled_start) AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Manila') = $1`, businessDate.Format("2006-01-02")).Scan(&count)
	return count, err
}

func (r *reportExportRepoImpl) CountSalaryCompletedBookingsMissingActualEnd(ctx context.Context, startDate time.Time, endDate time.Time) (int, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE status = 'completed'
		  AND actual_end IS NULL
		  AND DATE((COALESCE(actual_start, scheduled_start) AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Manila') BETWEEN $1 AND $2`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).Scan(&count)
	return count, err
}

func (r *reportExportRepoImpl) GetDailySalesRemittance(ctx context.Context, businessDate time.Time, branchID int64) (*model.DailySalesRemittance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	remittance := model.DailySalesRemittance{}
	err := r.db.QueryRow(ctx, remittanceSelectSQL+` WHERE business_date = $1 AND branch_id = $2`, businessDate.Format("2006-01-02"), branchID).Scan(remittanceScanTargets(&remittance)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fillRemittanceDates(&remittance)
	return &remittance, nil
}

func (r *reportExportRepoImpl) UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.DailySalesRemittance{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO daily_sales_remittances (
			business_date, branch_id, bill_1000, bill_500, bill_200, bill_100, bill_50, bill_20, bill_10, bill_5, bill_1,
			actual_remitted, tips_total, client_funds_used, client_funds_added, remitted_to_mark, other_remitted_amount,
			remitted_to, others_deducted, others_added, notes, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
		ON CONFLICT (business_date, branch_id) DO UPDATE SET
			bill_1000 = EXCLUDED.bill_1000, bill_500 = EXCLUDED.bill_500, bill_200 = EXCLUDED.bill_200,
			bill_100 = EXCLUDED.bill_100, bill_50 = EXCLUDED.bill_50, bill_20 = EXCLUDED.bill_20,
			bill_10 = EXCLUDED.bill_10, bill_5 = EXCLUDED.bill_5, bill_1 = EXCLUDED.bill_1,
			actual_remitted = EXCLUDED.actual_remitted, tips_total = EXCLUDED.tips_total,
			client_funds_used = EXCLUDED.client_funds_used, client_funds_added = EXCLUDED.client_funds_added,
			remitted_to_mark = EXCLUDED.remitted_to_mark, other_remitted_amount = EXCLUDED.other_remitted_amount,
			remitted_to = EXCLUDED.remitted_to, others_deducted = EXCLUDED.others_deducted,
			others_added = EXCLUDED.others_added, notes = EXCLUDED.notes, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP
		RETURNING remittance_id, business_date, branch_id, bill_1000, bill_500, bill_200, bill_100, bill_50, bill_20, bill_10, bill_5, bill_1,
			actual_remitted, tips_total, client_funds_used, client_funds_added, remitted_to_mark, other_remitted_amount,
			COALESCE(remitted_to, ''), others_deducted, others_added, COALESCE(notes, ''), created_by, updated_by, created_at, updated_at`,
		remittance.BusinessDate.Format("2006-01-02"), remittance.BranchID, remittance.Bill1000, remittance.Bill500, remittance.Bill200, remittance.Bill100, remittance.Bill50,
		remittance.Bill20, remittance.Bill10, remittance.Bill5, remittance.Bill1, remittance.ActualRemitted, remittance.TipsTotal, remittance.ClientFundsUsed,
		remittance.ClientFundsAdded, remittance.RemittedToMark, remittance.OtherRemittedAmount, remittance.RemittedTo, remittance.OthersDeducted,
		remittance.OthersAdded, remittance.Notes, remittance.CreatedBy, remittance.UpdatedBy,
	).Scan(remittanceScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillRemittanceDates(&out)
	return &out, nil
}

func (r *reportExportRepoImpl) ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, adjustmentSelectSQL+`
		WHERE a.voided_at IS NULL
		  AND a.period_start = $1
		  AND a.period_end = $2
		  AND ($3::int IS NULL OR a.therapist_id = $3)
		ORDER BY u.full_name, a.adjustment_date, a.adjustment_id`, filter.StartDate.Format("2006-01-02"), filter.EndDate.Format("2006-01-02"), filter.TherapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PayrollAdjustment, 0)
	for rows.Next() {
		var item model.PayrollAdjustment
		if err := rows.Scan(adjustmentScanTargets(&item)...); err != nil {
			return nil, err
		}
		fillAdjustmentDates(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *reportExportRepoImpl) CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.PayrollAdjustment{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO therapist_payroll_adjustments (
			therapist_id, adjustment_date, period_start, period_end, type, category, amount, reason, cash_movement, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING adjustment_id, therapist_id, '' AS therapist_name, adjustment_date, period_start, period_end, type, category, amount, reason,
			cash_movement, created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		adjustment.TherapistID, adjustment.AdjustmentDate.Format("2006-01-02"), adjustment.PeriodStart.Format("2006-01-02"), adjustment.PeriodEnd.Format("2006-01-02"),
		adjustment.Type, adjustment.Category, adjustment.Amount, adjustment.Reason, adjustment.CashMovement, adjustment.CreatedBy, adjustment.UpdatedBy,
	).Scan(adjustmentScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillAdjustmentDates(&out)
	return &out, nil
}

func (r *reportExportRepoImpl) UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.PayrollAdjustment{}
	err := r.db.QueryRow(ctx, `
		UPDATE therapist_payroll_adjustments SET
			therapist_id=$2, adjustment_date=$3, period_start=$4, period_end=$5, type=$6, category=$7,
			amount=$8, reason=$9, cash_movement=$10, updated_by=$11, updated_at=CURRENT_TIMESTAMP
		WHERE adjustment_id=$1 AND voided_at IS NULL
		RETURNING adjustment_id, therapist_id, '' AS therapist_name, adjustment_date, period_start, period_end, type, category, amount, reason,
			cash_movement, created_by, updated_by, voided_by, voided_at, created_at, updated_at`,
		adjustment.AdjustmentID, adjustment.TherapistID, adjustment.AdjustmentDate.Format("2006-01-02"), adjustment.PeriodStart.Format("2006-01-02"),
		adjustment.PeriodEnd.Format("2006-01-02"), adjustment.Type, adjustment.Category, adjustment.Amount, adjustment.Reason, adjustment.CashMovement, adjustment.UpdatedBy,
	).Scan(adjustmentScanTargets(&out)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fillAdjustmentDates(&out)
	return &out, nil
}

func (r *reportExportRepoImpl) VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `UPDATE therapist_payroll_adjustments SET voided_at = CURRENT_TIMESTAMP, voided_by = $2, updated_by = $2, updated_at = CURRENT_TIMESTAMP WHERE adjustment_id = $1 AND voided_at IS NULL`, adjustmentID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *reportExportRepoImpl) ListSalaryBookingRows(ctx context.Context, filter model.SalaryReportFilter) ([]model.ReportSalaryBookingRow, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT b.therapist_id, u.full_name, DATE((b.actual_end AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Manila') AS business_date,
		       COALESCE(s.name, 'Service'), b.booking_id, b.duration_minutes,
		       COALESCE(b.final_total, 0), COALESCE(b.therapist_earnings, 0)
		FROM bookings b
		JOIN users u ON u.user_id = b.therapist_id
		LEFT JOIN services s ON s.service_id = b.service_id
		WHERE b.status = 'completed'
		  AND b.actual_end IS NOT NULL
		  AND b.therapist_id IS NOT NULL
		  AND DATE((b.actual_end AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Manila') BETWEEN $1 AND $2
		  AND ($3::int IS NULL OR b.therapist_id = $3)
		ORDER BY u.full_name, business_date, b.booking_id`, filter.StartDate.Format("2006-01-02"), filter.EndDate.Format("2006-01-02"), filter.TherapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ReportSalaryBookingRow, 0)
	for rows.Next() {
		var item model.ReportSalaryBookingRow
		if err := rows.Scan(&item.TherapistID, &item.TherapistName, &item.BusinessDate, &item.ServiceName, &item.BookingID, &item.DurationMinutes, &item.FinalTotal, &item.TherapistEarnings); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const remittanceSelectSQL = `SELECT remittance_id, business_date, branch_id, bill_1000, bill_500, bill_200, bill_100, bill_50, bill_20, bill_10, bill_5, bill_1,
	actual_remitted, tips_total, client_funds_used, client_funds_added, remitted_to_mark, other_remitted_amount,
	COALESCE(remitted_to, ''), others_deducted, others_added, COALESCE(notes, ''), created_by, updated_by, created_at, updated_at FROM daily_sales_remittances`

const adjustmentSelectSQL = `SELECT a.adjustment_id, a.therapist_id, u.full_name, a.adjustment_date, a.period_start, a.period_end, a.type, a.category, a.amount, a.reason,
	a.cash_movement, a.created_by, a.updated_by, a.voided_by, a.voided_at, a.created_at, a.updated_at
	FROM therapist_payroll_adjustments a JOIN users u ON u.user_id = a.therapist_id `

func remittanceScanTargets(r *model.DailySalesRemittance) []any {
	return []any{&r.RemittanceID, &r.BusinessDate, &r.BranchID, &r.Bill1000, &r.Bill500, &r.Bill200, &r.Bill100, &r.Bill50, &r.Bill20, &r.Bill10, &r.Bill5, &r.Bill1, &r.ActualRemitted, &r.TipsTotal, &r.ClientFundsUsed, &r.ClientFundsAdded, &r.RemittedToMark, &r.OtherRemittedAmount, &r.RemittedTo, &r.OthersDeducted, &r.OthersAdded, &r.Notes, &r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt}
}

func adjustmentScanTargets(a *model.PayrollAdjustment) []any {
	return []any{&a.AdjustmentID, &a.TherapistID, &a.TherapistName, &a.AdjustmentDate, &a.PeriodStart, &a.PeriodEnd, &a.Type, &a.Category, &a.Amount, &a.Reason, &a.CashMovement, &a.CreatedBy, &a.UpdatedBy, &a.VoidedBy, &a.VoidedAt, &a.CreatedAt, &a.UpdatedAt}
}

func fillRemittanceDates(r *model.DailySalesRemittance) {
	r.Date = r.BusinessDate.Format("2006-01-02")
}

func fillAdjustmentDates(a *model.PayrollAdjustment) {
	a.Date = a.AdjustmentDate.Format("2006-01-02")
	a.PeriodStartDate = a.PeriodStart.Format("2006-01-02")
	a.PeriodEndDate = a.PeriodEnd.Format("2006-01-02")
}
