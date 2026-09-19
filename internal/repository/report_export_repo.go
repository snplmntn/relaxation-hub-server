package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type ReportExportRepository interface {
	ListActiveBranchTherapists(ctx context.Context) ([]model.ReportTherapistRosterRow, error)
	ListBookingExportRows(ctx context.Context, filter model.BookingExportFilter) ([]model.ReportBookingExportRow, error)
	ListDailySalesBookingRows(ctx context.Context, businessDate time.Time) ([]model.ReportDailySalesBookingRow, error)
	CountDailySalesCompletedBookingsMissingActualEnd(ctx context.Context, businessDate time.Time) (int, error)
	CountSalaryCompletedBookingsMissingActualEnd(ctx context.Context, startDate time.Time, endDate time.Time) (int, error)
	GetDailySalesRemittance(ctx context.Context, businessDate time.Time, branchID int64) (*model.DailySalesRemittance, error)
	UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error)
	// ListAccountingDayLineItems returns the accounting-sheet line items recorded
	// for a business date, keyed by branch. The daily sales report derives
	// tips_total/others_deducted/others_added from these on read.
	ListAccountingDayLineItems(ctx context.Context, businessDate time.Time) (map[int64]model.AccountingDayLineItems, error)
	ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error)
	CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error
	ListSalaryBookingRows(ctx context.Context, filter model.SalaryReportFilter) ([]model.ReportSalaryBookingRow, error)
}

func (r *reportExportRepoImpl) ListBookingExportRows(ctx context.Context, filter model.BookingExportFilter) ([]model.ReportBookingExportRow, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT b.booking_id,
		       business_day(b.scheduled_start) AS business_date,
		       b.therapist_id,
		       COALESCE(therapist.full_name, 'Unknown Therapist'),
		       COALESCE(client.full_name, 'Unknown Client'),
		       booked_service.service_name,
		       booked_service.duration_minutes,
		       COALESCE(b.duration_minutes, 0) AS booking_duration_minutes,
		       booked_service.duration_weight,
		       booked_service.duration_allocated,
		       booked_service.price_weight,
		       booked_service.commission_rate,
		       booked_service.service_number > 1 AS additional_service,
		       COALESCE(b.payment_method, ''),
		       COALESCE(b.final_total, 0),
		       COALESCE(b.therapist_earnings, 0)
		FROM bookings b
		JOIN users therapist ON therapist.user_id = b.therapist_id
		JOIN users client ON client.user_id = b.client_id
		LEFT JOIN services primary_service ON primary_service.service_id = b.service_id
		JOIN LATERAL (
			SELECT bs.position,
			       COALESCE(service.name, 'Service') AS service_name,
			       COALESCE(bs.allocated_duration_minutes, bs.duration_snapshot) AS duration_minutes,
			       bs.duration_snapshot AS duration_weight,
			       bs.allocated_duration_minutes IS NOT NULL AS duration_allocated,
			       bs.price_snapshot AS price_weight,
			       COALESCE(service.therapist_commission, 0) AS commission_rate,
			       ROW_NUMBER() OVER (ORDER BY bs.position, bs.booking_service_id) AS service_number
			FROM booking_services bs
			LEFT JOIN services service ON service.service_id = bs.service_id
			WHERE bs.booking_id = b.booking_id

			UNION ALL

			SELECT 0,
			       COALESCE(primary_service.name, 'Service'),
			       COALESCE(b.duration_minutes, 0),
			       COALESCE(b.duration_minutes, 0),
			       TRUE,
			       COALESCE(primary_service.base_price, b.final_total, 0),
			       COALESCE(primary_service.therapist_commission, 0),
			       1::bigint
			WHERE NOT EXISTS (
				SELECT 1 FROM booking_services existing WHERE existing.booking_id = b.booking_id
			)
		) booked_service ON TRUE
		WHERE b.status = 'completed'
		  AND b.actual_end IS NOT NULL
		  AND b.therapist_id IS NOT NULL
		  AND business_day(b.scheduled_start) BETWEEN $1 AND $2
		  AND ($3::int IS NULL OR b.therapist_id = $3)
		ORDER BY business_date, therapist.full_name, b.booking_id, booked_service.position`,
		filter.StartDate.Format("2006-01-02"), filter.EndDate.Format("2006-01-02"), filter.TherapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ReportBookingExportRow, 0)
	for rows.Next() {
		var item model.ReportBookingExportRow
		if err := rows.Scan(
			&item.BookingID,
			&item.BusinessDate,
			&item.TherapistID,
			&item.TherapistName,
			&item.ClientName,
			&item.ServiceName,
			&item.DurationMinutes,
			&item.BookingDurationMinutes,
			&item.ServiceDurationWeight,
			&item.DurationAllocated,
			&item.ServicePriceWeight,
			&item.ServiceCommissionRate,
			&item.AdditionalService,
			&item.PaymentMethod,
			&item.FinalTotal,
			&item.TherapistEarnings,
		); err != nil {
			return nil, err
		}
		item.Date = item.BusinessDate.Format("2006-01-02")
		items = append(items, item)
	}
	normalizeBookingExportDurations(items)
	allocateBookingExportAmounts(items)
	return items, rows.Err()
}

func normalizeBookingExportDurations(items []model.ReportBookingExportRow) {
	bookingRows := make(map[int64][]int)
	for i := range items {
		bookingRows[items[i].BookingID] = append(bookingRows[items[i].BookingID], i)
	}

	for _, indexes := range bookingRows {
		unallocated := make([]int, 0, len(indexes))
		weights := make([]float64, 0, len(indexes))
		allocatedMinutes := 0
		for _, index := range indexes {
			if items[index].DurationAllocated {
				allocatedMinutes += items[index].DurationMinutes
				continue
			}
			unallocated = append(unallocated, index)
			weights = append(weights, math.Max(0, items[index].ServiceDurationWeight))
		}
		if len(unallocated) == 0 {
			continue
		}

		remainingMinutes := max(0, items[indexes[0]].BookingDurationMinutes-allocatedMinutes)
		durations := allocateWholeUnits(remainingMinutes, weights)
		for i, index := range unallocated {
			items[index].DurationMinutes = durations[i]
		}
	}
}

func allocateWholeUnits(total int, weights []float64) []int {
	allocations := make([]int, len(weights))
	if len(weights) == 0 {
		return allocations
	}
	totalWeight := sumFloat64(weights)
	if totalWeight == 0 {
		for i := range weights {
			weights[i] = 1
		}
		totalWeight = float64(len(weights))
	}

	cumulativeWeight := 0.0
	allocated := 0
	for i, weight := range weights {
		cumulativeWeight += weight
		target := int(math.Round(float64(total) * cumulativeWeight / totalWeight))
		allocations[i] = target - allocated
		allocated = target
	}
	return allocations
}

func allocateBookingExportAmounts(items []model.ReportBookingExportRow) {
	bookingRows := make(map[int64][]int)
	for i := range items {
		bookingRows[items[i].BookingID] = append(bookingRows[items[i].BookingID], i)
	}

	for _, indexes := range bookingRows {
		if len(indexes) < 2 {
			continue
		}
		weights := make([]float64, len(indexes))
		for i, index := range indexes {
			weights[i] = math.Max(0, items[index].ServicePriceWeight)
		}
		if sumFloat64(weights) == 0 {
			for i, index := range indexes {
				weights[i] = float64(items[index].DurationMinutes)
			}
		}

		finalTotals := allocateMoney(items[indexes[0]].FinalTotal, weights)
		earnings := make([]float64, len(indexes))
		useServiceCommissions := true
		for _, index := range indexes {
			if items[index].ServiceCommissionRate <= 0 || items[index].ServiceDurationWeight <= 0 {
				useServiceCommissions = false
				break
			}
		}
		if useServiceCommissions {
			for i, index := range indexes {
				earnings[i] = math.Round(
					items[index].ServiceCommissionRate*float64(items[index].DurationMinutes)/items[index].ServiceDurationWeight*100,
				) / 100
			}
		} else {
			earnings = allocateMoney(items[indexes[0]].TherapistEarnings, weights)
		}
		for i, index := range indexes {
			items[index].FinalTotal = finalTotals[i]
			items[index].TherapistEarnings = earnings[i]
		}
	}
}

func allocateMoney(total float64, weights []float64) []float64 {
	allocations := make([]float64, len(weights))
	if len(weights) == 0 {
		return allocations
	}
	totalWeight := sumFloat64(weights)
	if totalWeight == 0 {
		for i := range weights {
			weights[i] = 1
		}
		totalWeight = float64(len(weights))
	}

	totalCents := math.Round(total * 100)
	cumulativeWeight := 0.0
	allocatedCents := 0.0
	for i, weight := range weights {
		cumulativeWeight += weight
		targetCents := math.Round(totalCents * cumulativeWeight / totalWeight)
		allocations[i] = (targetCents - allocatedCents) / 100
		allocatedCents = targetCents
	}
	return allocations
}

func sumFloat64(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
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
		  AND business_day(b.scheduled_start) = $1
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
		  AND business_day(scheduled_start) = $1`, businessDate.Format("2006-01-02")).Scan(&count)
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
		  AND business_day(scheduled_start) BETWEEN $1 AND $2`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).Scan(&count)
	return count, err
}

func (r *reportExportRepoImpl) GetDailySalesRemittance(ctx context.Context, businessDate time.Time, branchID int64) (*model.DailySalesRemittance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	remittance := model.DailySalesRemittance{}
	err := r.db.QueryRow(ctx, remittanceSelectSQL+` WHERE dsr.business_date = $1 AND dsr.branch_id = $2`, businessDate.Format("2006-01-02"), branchID).Scan(remittanceScanTargets(&remittance)...)
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

	// The upsert is wrapped in a CTE so vault_claimed_by_name can be resolved
	// from users in the same round trip. vault_claimed_at/_by are derived here
	// rather than taken from the payload: they are only stamped on the
	// false -> true transition and cleared on true -> false, so a repeated save
	// with vault_claimed already true never rewrites the original hand-off.
	out := model.DailySalesRemittance{}
	err := r.db.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO daily_sales_remittances (
				business_date, branch_id, bill_1000, bill_500, bill_200, bill_100, bill_50, bill_20, bill_10, bill_5, bill_1,
				actual_remitted, tips_total, client_funds_used, client_funds_added, remitted_to_mark, other_remitted_amount,
				remitted_to, others_deducted, others_added, notes, created_by, updated_by,
				gcash_on_hand, maya_on_hand, vault_claimed, vault_claimed_at, vault_claimed_by,
				closing_bill_1000, closing_bill_500, closing_bill_200, closing_bill_100, closing_bill_50,
				closing_bill_20, closing_bill_10, closing_bill_5, closing_bill_1
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,
				$24,$25,$26::boolean,
				CASE WHEN $26::boolean THEN NOW() ELSE NULL END,
				CASE WHEN $26::boolean THEN $27::int ELSE NULL END,
				$28,$29,$30,$31,$32,$33,$34,$35,$36
			)
			ON CONFLICT (business_date, branch_id) DO UPDATE SET
				bill_1000 = EXCLUDED.bill_1000, bill_500 = EXCLUDED.bill_500, bill_200 = EXCLUDED.bill_200,
				bill_100 = EXCLUDED.bill_100, bill_50 = EXCLUDED.bill_50, bill_20 = EXCLUDED.bill_20,
				bill_10 = EXCLUDED.bill_10, bill_5 = EXCLUDED.bill_5, bill_1 = EXCLUDED.bill_1,
				actual_remitted = EXCLUDED.actual_remitted, tips_total = EXCLUDED.tips_total,
				client_funds_used = EXCLUDED.client_funds_used, client_funds_added = EXCLUDED.client_funds_added,
				remitted_to_mark = EXCLUDED.remitted_to_mark, other_remitted_amount = EXCLUDED.other_remitted_amount,
				remitted_to = EXCLUDED.remitted_to, others_deducted = EXCLUDED.others_deducted,
				others_added = EXCLUDED.others_added, notes = EXCLUDED.notes,
				gcash_on_hand = EXCLUDED.gcash_on_hand, maya_on_hand = EXCLUDED.maya_on_hand,
				vault_claimed = EXCLUDED.vault_claimed,
				vault_claimed_at = CASE
					WHEN EXCLUDED.vault_claimed AND NOT daily_sales_remittances.vault_claimed THEN NOW()
					WHEN NOT EXCLUDED.vault_claimed THEN NULL
					ELSE daily_sales_remittances.vault_claimed_at
				END,
				vault_claimed_by = CASE
					WHEN EXCLUDED.vault_claimed AND NOT daily_sales_remittances.vault_claimed THEN $27::int
					WHEN NOT EXCLUDED.vault_claimed THEN NULL
					ELSE daily_sales_remittances.vault_claimed_by
				END,
				closing_bill_1000 = EXCLUDED.closing_bill_1000, closing_bill_500 = EXCLUDED.closing_bill_500,
				closing_bill_200 = EXCLUDED.closing_bill_200, closing_bill_100 = EXCLUDED.closing_bill_100,
				closing_bill_50 = EXCLUDED.closing_bill_50, closing_bill_20 = EXCLUDED.closing_bill_20,
				closing_bill_10 = EXCLUDED.closing_bill_10, closing_bill_5 = EXCLUDED.closing_bill_5,
				closing_bill_1 = EXCLUDED.closing_bill_1,
				updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP
			RETURNING *
		)
		SELECT `+remittanceSelectColumns+`
		FROM upserted dsr
		LEFT JOIN users vault_user ON vault_user.user_id = dsr.vault_claimed_by`,
		remittance.BusinessDate.Format("2006-01-02"), remittance.BranchID, remittance.Bill1000, remittance.Bill500, remittance.Bill200, remittance.Bill100, remittance.Bill50,
		remittance.Bill20, remittance.Bill10, remittance.Bill5, remittance.Bill1, remittance.ActualRemitted, remittance.TipsTotal, remittance.ClientFundsUsed,
		remittance.ClientFundsAdded, remittance.RemittedToMark, remittance.OtherRemittedAmount, remittance.RemittedTo, remittance.OthersDeducted,
		remittance.OthersAdded, remittance.Notes, remittance.CreatedBy, remittance.UpdatedBy,
		remittance.GCashOnHand, remittance.MayaOnHand, remittance.VaultClaimed, remittance.VaultClaimedBy,
		remittance.ClosingBill1000, remittance.ClosingBill500, remittance.ClosingBill200, remittance.ClosingBill100, remittance.ClosingBill50,
		remittance.ClosingBill20, remittance.ClosingBill10, remittance.ClosingBill5, remittance.ClosingBill1,
	).Scan(remittanceScanTargets(&out)...)
	if err != nil {
		return nil, err
	}
	fillRemittanceDates(&out)
	return &out, nil
}

func (r *reportExportRepoImpl) ListAccountingDayLineItems(ctx context.Context, businessDate time.Time) (map[int64]model.AccountingDayLineItems, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT branch_id, 'expense'::text AS kind, amount FROM accounting_expenses WHERE business_date = $1
		UNION ALL
		SELECT branch_id, 'tip'::text AS kind, amount FROM accounting_tips WHERE business_date = $1`,
		businessDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("query accounting day line items: %w", err)
	}
	defer rows.Close()

	byBranch := make(map[int64]model.AccountingDayLineItems)
	for rows.Next() {
		var branchID int64
		var kind string
		var amount float64
		if err := rows.Scan(&branchID, &kind, &amount); err != nil {
			return nil, fmt.Errorf("scan accounting day line item: %w", err)
		}
		items := byBranch[branchID]
		if kind == "tip" {
			items.TipAmounts = append(items.TipAmounts, amount)
		} else {
			items.ExpenseAmounts = append(items.ExpenseAmounts, amount)
		}
		byBranch[branchID] = items
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounting day line items: %w", err)
	}
	return byBranch, nil
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
		SELECT b.therapist_id, u.full_name, business_day(b.scheduled_start) AS business_date,
		       COALESCE(s.name, 'Service'), b.booking_id, b.duration_minutes,
		       COALESCE(b.final_total, 0), COALESCE(b.therapist_earnings, 0)
		FROM bookings b
		JOIN users u ON u.user_id = b.therapist_id
		LEFT JOIN services s ON s.service_id = b.service_id
		WHERE b.status = 'completed'
		  AND b.actual_end IS NOT NULL
		  AND b.therapist_id IS NOT NULL
		  AND business_day(b.scheduled_start) BETWEEN $1 AND $2
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

// remittanceSelectColumns expects the remittance row aliased as "dsr" and the
// vault-claiming user LEFT JOINed as "vault_user".
const remittanceSelectColumns = `dsr.remittance_id, dsr.business_date, dsr.branch_id, dsr.bill_1000, dsr.bill_500, dsr.bill_200,
	dsr.bill_100, dsr.bill_50, dsr.bill_20, dsr.bill_10, dsr.bill_5, dsr.bill_1,
	dsr.actual_remitted, dsr.tips_total, dsr.client_funds_used, dsr.client_funds_added, dsr.remitted_to_mark, dsr.other_remitted_amount,
	COALESCE(dsr.remitted_to, ''), dsr.others_deducted, dsr.others_added, COALESCE(dsr.notes, ''),
	dsr.gcash_on_hand, dsr.maya_on_hand, dsr.vault_claimed, dsr.vault_claimed_at, dsr.vault_claimed_by, COALESCE(vault_user.full_name, ''),
	dsr.closing_bill_1000, dsr.closing_bill_500, dsr.closing_bill_200, dsr.closing_bill_100, dsr.closing_bill_50,
	dsr.closing_bill_20, dsr.closing_bill_10, dsr.closing_bill_5, dsr.closing_bill_1,
	dsr.created_by, dsr.updated_by, dsr.created_at, dsr.updated_at`

const remittanceSelectSQL = `SELECT ` + remittanceSelectColumns + `
	FROM daily_sales_remittances dsr
	LEFT JOIN users vault_user ON vault_user.user_id = dsr.vault_claimed_by`

const adjustmentSelectSQL = `SELECT a.adjustment_id, a.therapist_id, u.full_name, a.adjustment_date, a.period_start, a.period_end, a.type, a.category, a.amount, a.reason,
	a.cash_movement, a.created_by, a.updated_by, a.voided_by, a.voided_at, a.created_at, a.updated_at
	FROM therapist_payroll_adjustments a JOIN users u ON u.user_id = a.therapist_id `

func remittanceScanTargets(r *model.DailySalesRemittance) []any {
	return []any{
		&r.RemittanceID, &r.BusinessDate, &r.BranchID, &r.Bill1000, &r.Bill500, &r.Bill200, &r.Bill100, &r.Bill50, &r.Bill20, &r.Bill10, &r.Bill5, &r.Bill1,
		&r.ActualRemitted, &r.TipsTotal, &r.ClientFundsUsed, &r.ClientFundsAdded, &r.RemittedToMark, &r.OtherRemittedAmount,
		&r.RemittedTo, &r.OthersDeducted, &r.OthersAdded, &r.Notes,
		&r.GCashOnHand, &r.MayaOnHand, &r.VaultClaimed, &r.VaultClaimedAt, &r.VaultClaimedBy, &r.VaultClaimedByName,
		&r.ClosingBill1000, &r.ClosingBill500, &r.ClosingBill200, &r.ClosingBill100, &r.ClosingBill50,
		&r.ClosingBill20, &r.ClosingBill10, &r.ClosingBill5, &r.ClosingBill1,
		&r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt,
	}
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
