package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/mock"
)

func TestPayrollRepoCreateRateInsertsCompensationRate(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)
	actorID := int64(7)
	effectiveFrom := mustRepoPayrollDate(t, "2026-05-18")
	now := time.Now().UTC()

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into staff_compensation_rates") &&
			strings.Contains(sql, "daily_rate_cents") &&
			strings.Contains(sql, "overtime_multiplier") &&
			strings.Contains(sql, "returning")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actorArg, ok := args[7].(*int64)
		return len(args) == 8 &&
			fmt.Sprint(args[0]) == "12" &&
			fmt.Sprint(args[1]) == model.RoleRider &&
			fmt.Sprint(args[2]) == "120000" &&
			args[3] == 1.25 &&
			args[4] == "2026-05-18" &&
			fmt.Sprint(args[5]) == "<nil>" &&
			args[6] == "rate" &&
			ok && actorArg != nil && *actorArg == actorID
	})).Return(row).Once()
	row.On("Scan", payrollRateScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 99
		*args.Get(1).(*int64) = 12
		*args.Get(2).(*model.PayrollRole) = model.PayrollRoleRider
		*args.Get(3).(*model.PayrollMoneyCents) = 120000
		*args.Get(4).(*float64) = 1.25
		*args.Get(5).(*time.Time) = effectiveFrom
		*args.Get(10).(*time.Time) = now
		*args.Get(11).(*time.Time) = now
	}).Return(nil).Once()

	created, err := repo.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      effectiveFrom,
		Notes:              "rate",
		CreatedBy:          &actorID,
		UpdatedBy:          &actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.RateID != 99 || created.EffectiveFromDate != "2026-05-18" {
		t.Fatalf("unexpected created rate: %#v", created)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoCreateRateRejectsOverlappingRangeInTransaction(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	rows := new(MockRows)
	repo := NewPayrollRepository(mockDB)
	effectiveFrom := mustRepoPayrollDate(t, "2026-05-10")
	effectiveTo := mustRepoPayrollDate(t, "2026-05-20")
	existingFrom := mustRepoPayrollDate(t, "2026-05-01")
	existingTo := mustRepoPayrollDate(t, "2026-05-15")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	advisoryCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "pg_advisory_xact_lock")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == payrollCompensationRateAdvisoryLockKey(12)
	})).Return(pgconn.NewCommandTag("SELECT 1"), nil).Once()
	rangeCall := tx.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_compensation_rates") &&
			strings.Contains(sql, "for update")
	}), []interface{}{int64(12)}).Return(rows, nil).Once()
	rangeCall.NotBefore(advisoryCall)
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 41
		*args.Get(1).(*time.Time) = existingFrom
		*args.Get(2).(**time.Time) = &existingTo
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	created, err := repo.CreateCompensationRateAtomic(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      effectiveFrom,
		EffectiveTo:        &effectiveTo,
	})
	if !errors.Is(err, model.ErrInvalidPayrollRate) {
		t.Fatalf("expected invalid rate for overlap, got %v", err)
	}
	if created != nil {
		t.Fatalf("expected no created rate, got %#v", created)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestPayrollRepoCreateRateClosesOpenRateAndInsertsInTransaction(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	rows := new(MockRows)
	lockRow := new(MockRow)
	insertRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	actorID := int64(7)
	effectiveFrom := mustRepoPayrollDate(t, "2026-05-18")
	existingFrom := mustRepoPayrollDate(t, "2026-05-01")
	now := time.Now().UTC()

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	advisoryCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "pg_advisory_xact_lock")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == payrollCompensationRateAdvisoryLockKey(12)
	})).Return(pgconn.NewCommandTag("SELECT 1"), nil).Once()
	rangeCall := tx.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_compensation_rates") &&
			strings.Contains(sql, "for update")
	}), []interface{}{int64(12)}).Return(rows, nil).Once()
	rangeCall.NotBefore(advisoryCall)
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 41
		*args.Get(1).(*time.Time) = existingFrom
		*args.Get(2).(**time.Time) = nil
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "payroll_runs")
	}), []interface{}{int64(41)}).Return(lockRow).Once()
	lockRow.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = false
	}).Return(nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_compensation_rates") &&
			strings.Contains(sql, "effective_to is null") &&
			strings.Contains(sql, "not exists") &&
			strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 &&
			args[0] == int64(41) &&
			args[1] == "2026-05-17" &&
			args[2] == actorID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into staff_compensation_rates")
	}), mock.Anything).Return(insertRow).Once()
	insertRow.On("Scan", payrollRateScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 99
		*args.Get(1).(*int64) = 12
		*args.Get(2).(*model.PayrollRole) = model.PayrollRoleRider
		*args.Get(3).(*model.PayrollMoneyCents) = 120000
		*args.Get(4).(*float64) = 1.25
		*args.Get(5).(*time.Time) = effectiveFrom
		*args.Get(10).(*time.Time) = now
		*args.Get(11).(*time.Time) = now
	}).Return(nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	created, err := repo.CreateCompensationRateAtomic(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      effectiveFrom,
		CreatedBy:          &actorID,
		UpdatedBy:          &actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.RateID != 99 {
		t.Fatalf("unexpected created rate: %#v", created)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestPayrollRepoCreateRateAutoCloseMapsGuardedNoRowToLocked(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	rows := new(MockRows)
	lockBeforeCloseRow := new(MockRow)
	lockAfterCloseRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	actorID := int64(7)
	effectiveFrom := mustRepoPayrollDate(t, "2026-05-18")
	existingFrom := mustRepoPayrollDate(t, "2026-05-01")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "pg_advisory_xact_lock")
	}), mock.Anything).Return(pgconn.NewCommandTag("SELECT 1"), nil).Once()
	tx.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_compensation_rates") &&
			strings.Contains(sql, "for update")
	}), []interface{}{int64(12)}).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 41
		*args.Get(1).(*time.Time) = existingFrom
		*args.Get(2).(**time.Time) = nil
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "payroll_runs")
	}), []interface{}{int64(41)}).Return(lockBeforeCloseRow).Once()
	lockBeforeCloseRow.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = false
	}).Return(nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_compensation_rates") &&
			strings.Contains(sql, "not exists") &&
			strings.Contains(sql, "payroll_rows")
	}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "payroll_runs")
	}), []interface{}{int64(41)}).Return(lockAfterCloseRow).Once()
	lockAfterCloseRow.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	created, err := repo.CreateCompensationRateAtomic(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      effectiveFrom,
		CreatedBy:          &actorID,
		UpdatedBy:          &actorID,
	})
	if !errors.Is(err, model.ErrPayrollRateLocked) {
		t.Fatalf("expected locked rate, got %v", err)
	}
	if created != nil {
		t.Fatalf("expected no created rate, got %#v", created)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestPayrollRepoRateLockChecksApprovedOrPaidPayrollRows(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_compensation_rates") &&
			strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid")
	}), []interface{}{int64(41)}).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	locked, err := repo.IsCompensationRateLocked(context.Background(), 41)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !locked {
		t.Fatalf("expected rate to be locked")
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoUpdateAdjustmentMapsGuardedNoRowToLocked(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)
	lockRow := new(MockRow)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_payroll_adjustments") &&
			strings.Contains(sql, "payroll_adjustment_details") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid") &&
			strings.Contains(sql, "not exists")
	}), mock.Anything).Return(row).Once()
	row.On("Scan", payrollAdjustmentScanMockArgs()...).Return(pgx.ErrNoRows).Once()
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_adjustment_details") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid")
	}), []interface{}{int64(44)}).Return(lockRow).Once()
	lockRow.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	adjustment, err := repo.UpdateStaffPayrollAdjustment(context.Background(), model.StaffPayrollAdjustment{
		AdjustmentID:   44,
		UserID:         12,
		Role:           model.PayrollRoleRider,
		AdjustmentDate: mustRepoPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustRepoPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustRepoPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeAdd,
		Category:       "bonus",
		AmountCents:    100,
		Reason:         "Bonus",
	})
	if !errors.Is(err, model.ErrPayrollAdjustmentLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	if adjustment != nil {
		t.Fatalf("expected nil adjustment, got %#v", adjustment)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoCreateAdjustmentPersistsActorAudit(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)
	actorID := int64(7)
	now := time.Now().UTC()

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into staff_payroll_adjustments") &&
			strings.Contains(sql, "created_by") &&
			strings.Contains(sql, "updated_by")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actorArg, ok := args[10].(*int64)
		return len(args) == 11 &&
			fmt.Sprint(args[0]) == "22" &&
			fmt.Sprint(args[1]) == model.RoleRider &&
			fmt.Sprint(args[7]) == "5000" &&
			ok && actorArg != nil && *actorArg == actorID
	})).Return(row).Once()
	row.On("Scan", payrollAdjustmentScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 81
		*args.Get(1).(*int64) = 22
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleRider
		*args.Get(4).(*time.Time) = mustRepoPayrollDate(t, "2026-05-17")
		*args.Get(5).(*time.Time) = mustRepoPayrollDate(t, "2026-05-01")
		*args.Get(6).(*time.Time) = mustRepoPayrollDate(t, "2026-05-31")
		*args.Get(7).(*model.PayrollAdjustmentType) = model.PayrollAdjustmentTypeAdd
		*args.Get(8).(*string) = "bonus"
		*args.Get(9).(*model.PayrollMoneyCents) = 5000
		*args.Get(10).(*string) = "Bonus"
		*args.Get(12).(*time.Time) = now
		*args.Get(13).(*time.Time) = now
	}).Return(nil).Once()

	created, err := repo.CreateStaffPayrollAdjustment(context.Background(), model.StaffPayrollAdjustment{
		UserID:         22,
		Role:           model.PayrollRoleRider,
		AdjustmentDate: mustRepoPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustRepoPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustRepoPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeAdd,
		Category:       "bonus",
		AmountCents:    5000,
		Reason:         "Bonus",
		CreatedBy:      &actorID,
		UpdatedBy:      &actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.AdjustmentID != 81 {
		t.Fatalf("unexpected created adjustment: %#v", created)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoUpdateAdjustmentPersistsActorAudit(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)
	actorID := int64(8)
	now := time.Now().UTC()

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_payroll_adjustments") &&
			strings.Contains(sql, "updated_by = $12")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actorArg, ok := args[11].(*int64)
		return len(args) == 12 &&
			fmt.Sprint(args[0]) == "44" &&
			fmt.Sprint(args[1]) == "22" &&
			ok && actorArg != nil && *actorArg == actorID
	})).Return(row).Once()
	row.On("Scan", payrollAdjustmentScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 44
		*args.Get(1).(*int64) = 22
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleRider
		*args.Get(4).(*time.Time) = mustRepoPayrollDate(t, "2026-05-17")
		*args.Get(5).(*time.Time) = mustRepoPayrollDate(t, "2026-05-01")
		*args.Get(6).(*time.Time) = mustRepoPayrollDate(t, "2026-05-31")
		*args.Get(7).(*model.PayrollAdjustmentType) = model.PayrollAdjustmentTypeMinus
		*args.Get(8).(*string) = "deduction"
		*args.Get(9).(*model.PayrollMoneyCents) = 500
		*args.Get(10).(*string) = "Deduction"
		*args.Get(12).(*time.Time) = now
		*args.Get(13).(*time.Time) = now
	}).Return(nil).Once()

	updated, err := repo.UpdateStaffPayrollAdjustment(context.Background(), model.StaffPayrollAdjustment{
		AdjustmentID:   44,
		UserID:         22,
		Role:           model.PayrollRoleRider,
		AdjustmentDate: mustRepoPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustRepoPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustRepoPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeMinus,
		Category:       "deduction",
		AmountCents:    500,
		Reason:         "Deduction",
		UpdatedBy:      &actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.AdjustmentID != 44 {
		t.Fatalf("unexpected updated adjustment: %#v", updated)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoVoidAdjustmentMapsRowsAffectedToLockedOrNotFound(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update staff_payroll_adjustments") &&
			strings.Contains(sql, "voided_at") &&
			strings.Contains(sql, "payroll_adjustment_details") &&
			strings.Contains(sql, "not exists")
	}), []interface{}{int64(44), int64(7)}).Return(pgconn.NewCommandTag("UPDATE 0"), nil).Once()
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_adjustment_details") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid")
	}), []interface{}{int64(44)}).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	err := repo.VoidStaffPayrollAdjustment(context.Background(), 44, 7)
	if !errors.Is(err, model.ErrPayrollAdjustmentLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoUpsertStaffProfileWritesUnifiedProfile(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	branchID := int64(3)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into staff_profiles") &&
			strings.Contains(sql, "select") &&
			strings.Contains(sql, "role = 'admin'") &&
			strings.Contains(sql, "on conflict") &&
			strings.Contains(sql, "usual_branch_id") &&
			strings.Contains(sql, "usual_location_label")
	}), []interface{}{int64(12), &branchID, "Makati"}).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

	err := repo.UpsertStaffProfile(context.Background(), 12, &branchID, "Makati")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoUpsertStaffProfileRejectsNonAdminTarget(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into staff_profiles") &&
			strings.Contains(sql, "role = 'admin'")
	}), []interface{}{int64(12), (*int64)(nil), ""}).Return(pgconn.NewCommandTag("INSERT 0"), nil).Once()

	err := repo.UpsertStaffProfile(context.Background(), 12, nil, "")
	if !errors.Is(err, model.ErrInvalidPayrollRole) {
		t.Fatalf("expected invalid payroll role, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoListPayrollAttendanceSourcesScansPayrollInputs(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	rows := new(MockRows)
	workDate := mustRepoPayrollDate(t, "2026-05-18")
	now := time.Now().UTC()

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_attendance_entries") &&
			strings.Contains(sql, "users") &&
			strings.Contains(sql, "rider_profiles") &&
			strings.Contains(sql, "staff_profiles")
	}), mock.Anything).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 701
		*args.Get(1).(*int64) = 12
		*args.Get(2).(*model.PayrollRole) = model.PayrollRoleRider
		*args.Get(3).(*string) = "Rider"
		*args.Get(5).(*string) = "Makati"
		*args.Get(6).(*time.Time) = workDate
		*args.Get(9).(*time.Time) = now
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	items, err := repo.ListPayrollAttendanceSources(context.Background(), workDate, workDate)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 1 || items[0].AttendanceID != 701 || items[0].Role != model.PayrollRoleRider {
		t.Fatalf("unexpected attendance sources: %#v", items)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoCreatePayrollRunInsertsDraftPayrollRun(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")
	now := time.Now().UTC()
	actorID := int64(7)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_runs") &&
			strings.Contains(sql, "period_start") &&
			strings.Contains(sql, "generated_by") &&
			strings.Contains(sql, "returning")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actorArg, ok := args[3].(*int64)
		return len(args) == 4 &&
			args[0] == "2026-05-01" &&
			args[1] == "2026-05-31" &&
			fmt.Sprint(args[2]) == model.PayrollRunStatusDraft &&
			ok && actorArg != nil && *actorArg == actorID
	})).Return(row).Once()
	row.On("Scan", payrollRunScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 801
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*model.PayrollRunStatus) = model.PayrollRunStatusDraft
		*args.Get(4).(**int64) = &actorID
		*args.Get(5).(*time.Time) = now
	}).Return(nil).Once()

	created, err := repo.CreatePayrollRun(context.Background(), model.PayrollRun{
		PeriodStart: start,
		PeriodEnd:   end,
		Status:      model.PayrollRunStatusDraft,
		GeneratedBy: &actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.PayrollRunID != 801 || created.StartDate != "2026-05-01" || created.EndDate != "2026-05-31" {
		t.Fatalf("unexpected created run: %#v", created)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoPersistDraftPayrollRunUsesTransactionAndReturnsDetailSnapshots(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	runRow := new(MockRow)
	payrollRow := new(MockRow)
	attendanceDetailRow := new(MockRow)
	bookingDetailRow := new(MockRow)
	adjustmentDetailRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")
	workDate := mustRepoPayrollDate(t, "2026-05-18")
	now := time.Now().UTC()
	actorID := int64(7)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	lockCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "pg_advisory_xact_lock")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == payrollRunPeriodAdvisoryLockKey(start, end)
	})).Return(pgconn.NewCommandTag("SELECT 1"), nil).Once()
	runCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_runs") &&
			strings.Contains(sql, "returning")
	}), mock.Anything).Return(runRow).Once()
	runCall.NotBefore(lockCall)
	runRow.On("Scan", payrollRunScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 801
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*model.PayrollRunStatus) = model.PayrollRunStatusDraft
		*args.Get(4).(**int64) = &actorID
		*args.Get(5).(*time.Time) = now
	}).Return(nil).Once()
	rowCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_rows") &&
			strings.Contains(sql, "coalesce(payment_method, '')") &&
			strings.Contains(sql, "returning")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 16 && args[0] == int64(801) && args[1] == int64(12)
	})).Return(payrollRow).Once()
	rowCall.NotBefore(runCall)
	payrollRow.On("Scan", payrollRowScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 901
		*args.Get(1).(*int64) = 801
		*args.Get(2).(*int64) = 12
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(4).(*string) = "Admin"
		*args.Get(7).(*model.PayrollRowStatus) = model.PayrollRowStatusDraft
		*args.Get(12).(*model.PayrollMoneyCents) = 1000
		*args.Get(15).(*model.PayrollMoneyCents) = 1000
		*args.Get(16).(*[]string) = []string{}
		*args.Get(19).(*model.PayrollPaymentMethod) = ""
		*args.Get(23).(*time.Time) = now
		*args.Get(24).(*time.Time) = now
	}).Return(nil).Once()
	attendanceCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_attendance_details") &&
			strings.Contains(sql, "returning detail_id")
	}), mock.Anything).Return(attendanceDetailRow).Once()
	attendanceCall.NotBefore(rowCall)
	attendanceDetailRow.On("Scan", payrollAttendanceDetailScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 1001
		*args.Get(1).(*int64) = 901
		*args.Get(2).(*int64) = 701
		*args.Get(3).(*time.Time) = workDate
		*args.Get(6).(*int) = 480
		*args.Get(7).(*int) = 480
		*args.Get(11).(*model.PayrollMoneyCents) = 1000
		*args.Get(12).(*time.Time) = now
		*args.Get(13).(*time.Time) = now
	}).Return(nil).Once()
	bookingCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_booking_details") &&
			strings.Contains(sql, "returning detail_id")
	}), mock.Anything).Return(bookingDetailRow).Once()
	bookingCall.NotBefore(attendanceCall)
	bookingDetailRow.On("Scan", payrollBookingDetailScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 1002
		*args.Get(1).(*int64) = 901
		*args.Get(2).(*int64) = 702
		*args.Get(3).(*time.Time) = workDate
		*args.Get(4).(*string) = "Massage"
		*args.Get(5).(*int) = 90
		*args.Get(6).(*model.PayrollMoneyCents) = 2000
		*args.Get(7).(*model.PayrollMoneyCents) = 1000
		*args.Get(8).(*time.Time) = now
		*args.Get(9).(*time.Time) = now
	}).Return(nil).Once()
	adjustmentCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into payroll_adjustment_details") &&
			strings.Contains(sql, "returning detail_id")
	}), mock.Anything).Return(adjustmentDetailRow).Once()
	adjustmentCall.NotBefore(bookingCall)
	adjustmentDetailRow.On("Scan", payrollAdjustmentDetailScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 1003
		*args.Get(1).(*int64) = 901
		*args.Get(2).(*int64) = 703
		*args.Get(3).(*time.Time) = workDate
		*args.Get(4).(*model.PayrollAdjustmentType) = model.PayrollAdjustmentTypeAdd
		*args.Get(5).(*string) = "bonus"
		*args.Get(6).(*model.PayrollMoneyCents) = 1000
		*args.Get(7).(*string) = "Bonus"
		*args.Get(8).(*time.Time) = now
		*args.Get(9).(*time.Time) = now
	}).Return(nil).Once()
	voidCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "replaced_by_run_id") &&
			strings.Contains(sql, "payroll_run_id <> $4")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 4 && args[0] == "2026-05-01" && args[1] == "2026-05-31" && args[2] == actorID && args[3] == int64(801)
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	voidCall.NotBefore(adjustmentCall)
	commitCall := tx.On("Commit", mock.Anything).Return(nil).Once()
	commitCall.NotBefore(voidCall)
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	persisted, err := repo.PersistDraftPayrollRun(context.Background(), model.PayrollRun{
		PeriodStart: start,
		PeriodEnd:   end,
		Status:      model.PayrollRunStatusDraft,
		GeneratedBy: &actorID,
		Rows: []model.PayrollRow{{
			UserID:           12,
			Role:             model.PayrollRoleAdmin,
			FullNameSnapshot: "Admin",
			Status:           model.PayrollRowStatusDraft,
			GrossCents:       1000,
			FinalPayCents:    1000,
			BlockerCodes:     []string{},
			AttendanceDetails: []model.PayrollAttendanceDetail{{
				AttendanceID:    701,
				WorkDate:        workDate,
				WorkedMinutes:   480,
				RegularMinutes:  480,
				GrossCents:      1000,
				SourceUpdatedAt: now,
			}},
			BookingDetails: []model.PayrollBookingDetail{{
				BookingID:              702,
				BusinessDate:           workDate,
				ServiceName:            "Massage",
				DurationMinutes:        90,
				FinalTotalCents:        2000,
				TherapistEarningsCents: 1000,
				SourceUpdatedAt:        now,
			}},
			AdjustmentDetails: []model.PayrollAdjustmentDetail{{
				AdjustmentID:    703,
				AdjustmentDate:  workDate,
				Type:            model.PayrollAdjustmentTypeAdd,
				Category:        "bonus",
				AmountCents:     1000,
				Reason:          "Bonus",
				SourceUpdatedAt: now,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if persisted.PayrollRunID != 801 || len(persisted.Rows) != 1 {
		t.Fatalf("unexpected persisted run: %#v", persisted)
	}
	gotRow := persisted.Rows[0]
	if gotRow.PayrollRunID != 801 || gotRow.PayrollRowID != 901 || gotRow.PaymentMethod != "" {
		t.Fatalf("unexpected persisted row: %#v", gotRow)
	}
	if len(gotRow.AttendanceDetails) != 1 || gotRow.AttendanceDetails[0].DetailID != 1001 || gotRow.AttendanceDetails[0].PayrollRowID != 901 {
		t.Fatalf("expected returned attendance detail IDs, got %#v", gotRow.AttendanceDetails)
	}
	if len(gotRow.BookingDetails) != 1 || gotRow.BookingDetails[0].DetailID != 1002 || gotRow.BookingDetails[0].PayrollRowID != 901 {
		t.Fatalf("expected returned booking detail IDs, got %#v", gotRow.BookingDetails)
	}
	if len(gotRow.AdjustmentDetails) != 1 || gotRow.AdjustmentDetails[0].DetailID != 1003 || gotRow.AdjustmentDetails[0].PayrollRowID != 901 {
		t.Fatalf("expected returned adjustment detail IDs, got %#v", gotRow.AdjustmentDetails)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoPersistDraftPayrollRunRollsBackWhenDetailInsertFails(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	runRow := new(MockRow)
	payrollRow := new(MockRow)
	detailRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")
	workDate := mustRepoPayrollDate(t, "2026-05-18")
	now := time.Now().UTC()
	actorID := int64(7)
	insertErr := errors.New("detail insert failed")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "pg_advisory_xact_lock")
	}), mock.Anything).Return(pgconn.NewCommandTag("SELECT 1"), nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into payroll_runs")
	}), mock.Anything).Return(runRow).Once()
	runRow.On("Scan", payrollRunScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 801
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*model.PayrollRunStatus) = model.PayrollRunStatusDraft
		*args.Get(4).(**int64) = &actorID
		*args.Get(5).(*time.Time) = now
	}).Return(nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into payroll_rows")
	}), mock.Anything).Return(payrollRow).Once()
	payrollRow.On("Scan", payrollRowScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 901
		*args.Get(1).(*int64) = 801
		*args.Get(2).(*int64) = 12
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(4).(*string) = "Admin"
		*args.Get(7).(*model.PayrollRowStatus) = model.PayrollRowStatusDraft
		*args.Get(16).(*[]string) = []string{}
		*args.Get(19).(*model.PayrollPaymentMethod) = ""
		*args.Get(23).(*time.Time) = now
		*args.Get(24).(*time.Time) = now
	}).Return(nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into payroll_attendance_details")
	}), mock.Anything).Return(detailRow).Once()
	detailRow.On("Scan", payrollAttendanceDetailScanMockArgs()...).Return(insertErr).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	persisted, err := repo.PersistDraftPayrollRun(context.Background(), model.PayrollRun{
		PeriodStart: start,
		PeriodEnd:   end,
		Status:      model.PayrollRunStatusDraft,
		GeneratedBy: &actorID,
		Rows: []model.PayrollRow{{
			UserID:           12,
			Role:             model.PayrollRoleAdmin,
			FullNameSnapshot: "Admin",
			Status:           model.PayrollRowStatusDraft,
			AttendanceDetails: []model.PayrollAttendanceDetail{{
				AttendanceID:    701,
				WorkDate:        workDate,
				SourceUpdatedAt: now,
			}},
		}},
	})
	if !errors.Is(err, insertErr) {
		t.Fatalf("expected detail insert error, got %v", err)
	}
	if persisted != nil {
		t.Fatalf("expected no persisted run, got %#v", persisted)
	}
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	tx.AssertNotCalled(t, "Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update payroll_runs")
	}), mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoListPayrollRowsCoalescesNullPaymentMethod(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB).(*payrollRepoImpl)
	rows := new(MockRows)
	now := time.Now().UTC()

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "from payroll_rows") &&
			strings.Contains(sql, "coalesce(payment_method, '')")
	}), []interface{}{int64(801)}).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", payrollRowScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 901
		*args.Get(1).(*int64) = 801
		*args.Get(2).(*int64) = 12
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(4).(*string) = "Admin"
		*args.Get(7).(*model.PayrollRowStatus) = model.PayrollRowStatusDraft
		*args.Get(16).(*[]string) = []string{}
		*args.Get(19).(*model.PayrollPaymentMethod) = ""
		*args.Get(23).(*time.Time) = now
		*args.Get(24).(*time.Time) = now
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "from payroll_attendance_details")
	}), []interface{}{int64(901)}).Return(emptyMockRows(), nil).Once()
	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "from payroll_booking_details")
	}), []interface{}{int64(901)}).Return(emptyMockRows(), nil).Once()
	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "from payroll_adjustment_details")
	}), []interface{}{int64(901)}).Return(emptyMockRows(), nil).Once()

	items, err := repo.listPayrollRows(context.Background(), 801)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 1 || items[0].PaymentMethod != "" {
		t.Fatalf("expected empty payment method from coalesced draft row, got %#v", items)
	}
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestPayrollRepoHasActivePayrollCoverageChecksApprovedOrPaidPayroll(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	row := new(MockRow)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_attendance_details") &&
			strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "approved") &&
			strings.Contains(sql, "paid") &&
			!strings.Contains(sql, "draft")
	}), []interface{}{"attendance", int64(701)}).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	covered, err := repo.HasActivePayrollCoverage(context.Background(), "attendance", 701)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !covered {
		t.Fatalf("expected source to be covered")
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoApprovePayrollRunApprovesRunAndCleanRows(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	runCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "status = 'approved'") &&
			strings.Contains(sql, "approved_by") &&
			strings.Contains(sql, "not exists") &&
			strings.Contains(sql, "blocker_codes")
	}), []interface{}{int64(71), int64(7)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	rowCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_rows") &&
			strings.Contains(sql, "status = 'approved'") &&
			strings.Contains(sql, "status = 'draft'")
	}), []interface{}{int64(71)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	rowCall.NotBefore(runCall)
	commitCall := tx.On("Commit", mock.Anything).Return(nil).Once()
	commitCall.NotBefore(rowCall)
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	if err := repo.ApprovePayrollRun(context.Background(), 71, 7); err != nil {
		t.Fatalf("expected approve to succeed, got %v", err)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoVoidPayrollRunVoidsRunAndRows(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	runCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "status = 'voided'") &&
			strings.Contains(sql, "status in ('draft', 'approved')") &&
			strings.Contains(sql, "voided_reason")
	}), []interface{}{int64(71), int64(7), "duplicate"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	rowCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_rows") &&
			strings.Contains(sql, "status = 'voided'") &&
			strings.Contains(sql, "'blocked'")
	}), []interface{}{int64(71)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	rowCall.NotBefore(runCall)
	commitCall := tx.On("Commit", mock.Anything).Return(nil).Once()
	commitCall.NotBefore(rowCall)
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	if err := repo.VoidPayrollRun(context.Background(), 71, 7, "duplicate"); err != nil {
		t.Fatalf("expected void to succeed, got %v", err)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoVoidPayrollRunRejectsPaidRows(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	lifecycleRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "not exists") &&
			strings.Contains(sql, "status = 'paid'") &&
			strings.Contains(sql, "ledger_entry_id is not null")
	}), []interface{}{int64(71), int64(7), "duplicate"}).Return(pgconn.NewCommandTag("UPDATE 0"), nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "from payroll_runs run")
	}), []interface{}{int64(71)}).Return(lifecycleRow).Once()
	lifecycleRow.On("Scan", mock.AnythingOfType("*model.PayrollRunStatus"), mock.AnythingOfType("*bool"), mock.AnythingOfType("*bool")).Run(func(args mock.Arguments) {
		*args.Get(0).(*model.PayrollRunStatus) = model.PayrollRunStatusApproved
		*args.Get(1).(*bool) = false
		*args.Get(2).(*bool) = true
	}).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	err := repo.VoidPayrollRun(context.Background(), 71, 7, "duplicate")
	if !errors.Is(err, model.ErrPayrollRunImmutable) {
		t.Fatalf("expected immutable error, got %v", err)
	}
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoMarkPayrollRowPaidGuardsApprovedRows(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_rows") &&
			strings.Contains(sql, "status = 'paid'") &&
			strings.Contains(sql, "paid_at") &&
			strings.Contains(sql, "status = 'approved'") &&
			strings.Contains(sql, "ledger_entry_id is null")
	}), []interface{}{int64(81), int64(7), "cash", "CASH-1", "notes", int64(909)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.MarkPayrollRowPaid(context.Background(), 81, 7, "cash", "CASH-1", "notes", 909)
	if err != nil {
		t.Fatalf("expected mark paid to succeed, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoRecordPayrollRowPaymentCreatesSettlementAndMarksPaidAtomically(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	lockRow := new(MockRow)
	ledgerRow := new(MockRow)
	paidRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")
	now := time.Now().UTC()

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	lockCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "payroll_runs") &&
			strings.Contains(sql, "payroll_rows") &&
			strings.Contains(sql, "for update")
	}), []interface{}{int64(71), int64(81)}).Return(lockRow).Once()
	lockRow.On("Scan", payrollPaymentLockScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*model.PayrollRunStatus) = model.PayrollRunStatusApproved
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*int64) = 81
		*args.Get(4).(*int64) = 71
		*args.Get(5).(*int64) = 22
		*args.Get(6).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(7).(*string) = "Admin One"
		*args.Get(8).(*model.PayrollMoneyCents) = 12345
		*args.Get(9).(**int64) = nil
		*args.Get(10).(*model.PayrollRowStatus) = model.PayrollRowStatusApproved
	}).Return(nil).Once()
	ledgerCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into ledger_entries") &&
			strings.Contains(sql, "payroll_run_id") &&
			strings.Contains(sql, "payroll_row_id") &&
			strings.Contains(sql, "target_role") &&
			strings.Contains(sql, "returning entry_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 11 &&
			args[0] == int64(71) &&
			args[1] == int64(81) &&
			args[2] == int64(22) &&
			args[3] == string(TargetRoleAdmin) &&
			args[4] == 123.45 &&
			args[5] == "cash" &&
			args[6] == "CASH-1" &&
			args[7] == int64(7) &&
			args[8] == "2026-05-01" &&
			args[9] == "2026-05-31" &&
			args[10] == "Admin One"
	})).Return(ledgerRow).Once()
	ledgerCall.NotBefore(lockCall)
	ledgerRow.On("Scan", mock.AnythingOfType("*int64")).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 909
	}).Return(nil).Once()
	paidCall := tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_rows") &&
			strings.Contains(sql, "status = 'paid'") &&
			strings.Contains(sql, "ledger_entry_id = $6") &&
			strings.Contains(sql, "returning payroll_row_id")
	}), []interface{}{int64(81), int64(7), "cash", "CASH-1", "notes", int64(909)}).Return(paidRow).Once()
	paidCall.NotBefore(ledgerCall)
	paidRow.On("Scan", payrollRowScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 81
		*args.Get(1).(*int64) = 71
		*args.Get(2).(*int64) = 22
		*args.Get(3).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(4).(*string) = "Admin One"
		*args.Get(7).(*model.PayrollRowStatus) = model.PayrollRowStatusPaid
		*args.Get(15).(*model.PayrollMoneyCents) = 12345
		*args.Get(16).(*[]string) = []string{}
		*args.Get(19).(*model.PayrollPaymentMethod) = model.PayrollPaymentMethodCash
		*args.Get(22).(**int64) = ptrInt64(909)
		*args.Get(23).(*time.Time) = now
		*args.Get(24).(*time.Time) = now
	}).Return(nil).Once()
	completeCall := tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "status = 'paid'") &&
			strings.Contains(sql, "status <> 'paid'")
	}), []interface{}{int64(71)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()
	completeCall.NotBefore(paidCall)
	commitCall := tx.On("Commit", mock.Anything).Return(nil).Once()
	commitCall.NotBefore(completeCall)
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	row, err := repo.RecordPayrollRowPayment(context.Background(), 71, 81, 7, "cash", "CASH-1", "notes")
	if err != nil {
		t.Fatalf("expected payment to succeed, got %v", err)
	}
	if row == nil || row.Status != model.PayrollRowStatusPaid || row.LedgerEntryID == nil || *row.LedgerEntryID != 909 {
		t.Fatalf("expected paid row with ledger entry, got %#v", row)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoRecordPayrollRowPaymentRollsBackWhenLedgerInsertFails(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	lockRow := new(MockRow)
	ledgerRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	insertErr := errors.New("ledger insert failed")
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "for update")
	}), []interface{}{int64(71), int64(81)}).Return(lockRow).Once()
	lockRow.On("Scan", payrollPaymentLockScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*model.PayrollRunStatus) = model.PayrollRunStatusApproved
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*int64) = 81
		*args.Get(4).(*int64) = 71
		*args.Get(5).(*int64) = 22
		*args.Get(6).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(7).(*string) = "Admin One"
		*args.Get(8).(*model.PayrollMoneyCents) = 12345
		*args.Get(9).(**int64) = nil
		*args.Get(10).(*model.PayrollRowStatus) = model.PayrollRowStatusApproved
	}).Return(nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into ledger_entries")
	}), mock.Anything).Return(ledgerRow).Once()
	ledgerRow.On("Scan", mock.AnythingOfType("*int64")).Return(insertErr).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	row, err := repo.RecordPayrollRowPayment(context.Background(), 71, 81, 7, "cash", "CASH-1", "notes")
	if !errors.Is(err, insertErr) {
		t.Fatalf("expected ledger insert error, got %v", err)
	}
	if row != nil {
		t.Fatalf("expected no paid row, got %#v", row)
	}
	tx.AssertNotCalled(t, "QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update payroll_rows")
	}), mock.Anything)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoRecordPayrollRowPaymentRejectsDuplicateSettlementBeforeInsert(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	lockRow := new(MockRow)
	repo := NewPayrollRepository(mockDB)
	start := mustRepoPayrollDate(t, "2026-05-01")
	end := mustRepoPayrollDate(t, "2026-05-31")
	existingLedgerID := int64(909)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "for update")
	}), []interface{}{int64(71), int64(81)}).Return(lockRow).Once()
	lockRow.On("Scan", payrollPaymentLockScanMockArgs()...).Run(func(args mock.Arguments) {
		*args.Get(0).(*model.PayrollRunStatus) = model.PayrollRunStatusApproved
		*args.Get(1).(*time.Time) = start
		*args.Get(2).(*time.Time) = end
		*args.Get(3).(*int64) = 81
		*args.Get(4).(*int64) = 71
		*args.Get(5).(*int64) = 22
		*args.Get(6).(*model.PayrollRole) = model.PayrollRoleAdmin
		*args.Get(7).(*string) = "Admin One"
		*args.Get(8).(*model.PayrollMoneyCents) = 12345
		*args.Get(9).(**int64) = &existingLedgerID
		*args.Get(10).(*model.PayrollRowStatus) = model.PayrollRowStatusPaid
	}).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	row, err := repo.RecordPayrollRowPayment(context.Background(), 71, 81, 7, "cash", "CASH-1", "notes")
	if !errors.Is(err, model.ErrPayrollRunImmutable) {
		t.Fatalf("expected immutable duplicate settlement error, got %v", err)
	}
	if row != nil {
		t.Fatalf("expected no paid row, got %#v", row)
	}
	tx.AssertNotCalled(t, "QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into ledger_entries")
	}), mock.Anything)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestPayrollRepoUpdatePayrollRunPaidIfCompleteRequiresAllRowsPaid(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update payroll_runs") &&
			strings.Contains(sql, "status = 'paid'") &&
			strings.Contains(sql, "status = 'approved'") &&
			strings.Contains(sql, "not exists") &&
			strings.Contains(sql, "status <> 'paid'")
	}), []interface{}{int64(71)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	if err := repo.UpdatePayrollRunPaidIfComplete(context.Background(), 71); err != nil {
		t.Fatalf("expected paid completion update to succeed, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestPayrollRepoCheckPayrollRunStalenessReturnsReasons(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	rows := new(MockRows)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "staff_attendance_entries") &&
			strings.Contains(sql, "bookings") &&
			strings.Contains(sql, "staff_payroll_adjustments") &&
			strings.Contains(sql, "source_updated_at")
	}), []interface{}{int64(71)}).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.AnythingOfType("*string")).Run(func(args mock.Arguments) {
		*args.Get(0).(*string) = "attendance_source_updated"
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	stale, reasons, err := repo.CheckPayrollRunStaleness(context.Background(), 71)
	if err != nil {
		t.Fatalf("expected staleness check to succeed, got %v", err)
	}
	if !stale || !sameRepoStrings(reasons, []string{"attendance_source_updated"}) {
		t.Fatalf("expected stale attendance reason, stale=%v reasons=%#v", stale, reasons)
	}
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestPayrollRepoCheckPayrollRunStalenessDetectsNewEligibleSources(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewPayrollRepository(mockDB)
	rows := new(MockRows)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "new_attendance_source") &&
			strings.Contains(sql, "new_booking_source") &&
			strings.Contains(sql, "new_adjustment_source") &&
			strings.Contains(sql, "sae.work_date between run.period_start and run.period_end") &&
			strings.Contains(sql, "u.role in ('rider', 'admin')") &&
			strings.Contains(sql, "b.status = 'completed'") &&
			strings.Contains(sql, "spa.voided_at is null") &&
			strings.Contains(sql, "not exists")
	}), []interface{}{int64(71)}).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.AnythingOfType("*string")).Run(func(args mock.Arguments) {
		*args.Get(0).(*string) = "new_attendance_source"
	}).Return(nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.AnythingOfType("*string")).Run(func(args mock.Arguments) {
		*args.Get(0).(*string) = "new_booking_source"
	}).Return(nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.AnythingOfType("*string")).Run(func(args mock.Arguments) {
		*args.Get(0).(*string) = "new_adjustment_source"
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	stale, reasons, err := repo.CheckPayrollRunStaleness(context.Background(), 71)
	if err != nil {
		t.Fatalf("expected staleness check to succeed, got %v", err)
	}
	if !stale || !sameRepoStrings(reasons, []string{"new_attendance_source", "new_booking_source", "new_adjustment_source"}) {
		t.Fatalf("expected new source reasons, stale=%v reasons=%#v", stale, reasons)
	}
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func payrollRateScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	}
}

func payrollAdjustmentScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	}
}

func payrollRunScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	}
}

func payrollRowScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything,
	}
}

func payrollAttendanceDetailScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	}
}

func payrollBookingDetailScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	}
}

func payrollAdjustmentDetailScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	}
}

func payrollPaymentLockScanMockArgs() []interface{} {
	return []interface{}{
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func sameRepoStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func emptyMockRows() *MockRows {
	rows := new(MockRows)
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	return rows
}

func mustRepoPayrollDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}
