package service

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/xuri/excelize/v2"
)

type fakePayrollRepo struct {
	openRate      *model.StaffCompensationRate
	openRateErr   error
	closeRateID   int64
	closeTo       time.Time
	closeActorID  int64
	closeRateErr  error
	createRate    *model.StaffCompensationRate
	rateLocked    bool
	rateLockedErr error

	adjustmentLocked    bool
	adjustmentLockedErr error
	createdAdjustment   *model.StaffPayrollAdjustment
	updatedAdjustment   *model.StaffPayrollAdjustment
	voidedAdjustmentID  int64
	voidedActorID       int64

	profileUserID        int64
	profileBranchID      *int64
	profileLocationLabel string

	attendanceSources []repository.PayrollAttendanceSource
	bookingSources    []repository.PayrollBookingSource
	adjustmentSources []repository.PayrollAdjustmentSource
	effectiveRates    map[int64]*model.StaffCompensationRate
	coverage          map[string]bool
	createdRun        *model.PayrollRun
	createdRows       []model.PayrollRow
	attendanceDetails []model.PayrollAttendanceDetail
	bookingDetails    []model.PayrollBookingDetail
	adjustmentDetails []model.PayrollAdjustmentDetail
	operationLog      []string
	voidStart         time.Time
	voidEnd           time.Time
	voidActorID       int64
	replacementRunID  int64

	runForUpdate        *model.PayrollRun
	listedRows          []model.PayrollRow
	approvedRunID       int64
	approveActorID      int64
	voidedRunID         int64
	voidedRunActorID    int64
	voidedRunReason     string
	paidRowID           int64
	paidBy              int64
	paidMethod          string
	paidReference       string
	paidNotes           string
	paidLedgerEntryID   int64
	atomicPaidRunID     int64
	atomicPaidRowID     int64
	atomicPaidBy        int64
	atomicPaidMethod    string
	atomicPaidReference string
	atomicPaidNotes     string
	runPaidIfCompleteID int64
	stale               bool
	staleReasons        []string
	runDetail           *model.PayrollRun
}

func (f *fakePayrollRepo) GetOpenCompensationRate(ctx context.Context, userID int64) (*model.StaffCompensationRate, error) {
	if f.openRateErr != nil {
		return nil, f.openRateErr
	}
	if f.openRate == nil {
		return nil, model.ErrNotFound
	}
	copied := *f.openRate
	return &copied, nil
}

func (f *fakePayrollRepo) CloseCompensationRate(ctx context.Context, rateID int64, effectiveTo time.Time, actorID int64) error {
	f.closeRateID = rateID
	f.closeTo = effectiveTo
	f.closeActorID = actorID
	return f.closeRateErr
}

func (f *fakePayrollRepo) CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error) {
	copied := rate
	copied.RateID = 99
	f.createRate = &copied
	return &copied, nil
}

func (f *fakePayrollRepo) CreateCompensationRateAtomic(ctx context.Context, rate model.StaffCompensationRate) (*model.StaffCompensationRate, error) {
	if f.rateLocked {
		return nil, model.ErrPayrollRateLocked
	}
	return f.CreateCompensationRate(ctx, rate)
}

func (f *fakePayrollRepo) IsCompensationRateLocked(ctx context.Context, rateID int64) (bool, error) {
	return f.rateLocked, f.rateLockedErr
}

func (f *fakePayrollRepo) ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error) {
	return nil, nil
}

func (f *fakePayrollRepo) UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error {
	f.profileUserID = userID
	f.profileBranchID = branchID
	f.profileLocationLabel = locationLabel
	return nil
}

func (f *fakePayrollRepo) ListStaffPayrollAdjustments(ctx context.Context, filter repository.StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error) {
	return nil, nil
}

func (f *fakePayrollRepo) CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error) {
	copied := adjustment
	copied.AdjustmentID = 55
	f.createdAdjustment = &copied
	return &copied, nil
}

func (f *fakePayrollRepo) UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment) (*model.StaffPayrollAdjustment, error) {
	copied := adjustment
	f.updatedAdjustment = &copied
	return &copied, nil
}

func (f *fakePayrollRepo) VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	f.voidedAdjustmentID = adjustmentID
	f.voidedActorID = actorID
	return nil
}

func (f *fakePayrollRepo) IsStaffPayrollAdjustmentLocked(ctx context.Context, adjustmentID int64) (bool, error) {
	return f.adjustmentLocked, f.adjustmentLockedErr
}

func (f *fakePayrollRepo) ListPayrollAttendanceSources(ctx context.Context, start, end time.Time) ([]repository.PayrollAttendanceSource, error) {
	return f.attendanceSources, nil
}

func (f *fakePayrollRepo) ListPayrollTherapistBookingSources(ctx context.Context, start, end time.Time) ([]repository.PayrollBookingSource, error) {
	return f.bookingSources, nil
}

func (f *fakePayrollRepo) ListPayrollAdjustmentSources(ctx context.Context, start, end time.Time) ([]repository.PayrollAdjustmentSource, error) {
	return f.adjustmentSources, nil
}

func (f *fakePayrollRepo) FindEffectiveRate(ctx context.Context, userID int64, workDate time.Time) (*model.StaffCompensationRate, error) {
	if f.effectiveRates == nil {
		return nil, model.ErrNotFound
	}
	rate := f.effectiveRates[userID]
	if rate == nil {
		return nil, model.ErrNotFound
	}
	copied := *rate
	return &copied, nil
}

func (f *fakePayrollRepo) CreatePayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error) {
	run.PayrollRunID = 700
	f.createdRun = &run
	f.operationLog = append(f.operationLog, "create_run")
	return &run, nil
}

func (f *fakePayrollRepo) CreatePayrollRow(ctx context.Context, row model.PayrollRow) (*model.PayrollRow, error) {
	row.PayrollRowID = int64(800 + len(f.createdRows))
	f.createdRows = append(f.createdRows, row)
	f.operationLog = append(f.operationLog, "create_row")
	return &row, nil
}

func (f *fakePayrollRepo) CreatePayrollAttendanceDetail(ctx context.Context, rowID int64, detail model.PayrollAttendanceDetail) error {
	detail.PayrollRowID = rowID
	f.attendanceDetails = append(f.attendanceDetails, detail)
	f.operationLog = append(f.operationLog, "attendance_detail")
	return nil
}

func (f *fakePayrollRepo) CreatePayrollBookingDetail(ctx context.Context, rowID int64, detail model.PayrollBookingDetail) error {
	detail.PayrollRowID = rowID
	f.bookingDetails = append(f.bookingDetails, detail)
	f.operationLog = append(f.operationLog, "booking_detail")
	return nil
}

func (f *fakePayrollRepo) CreatePayrollAdjustmentDetail(ctx context.Context, rowID int64, detail model.PayrollAdjustmentDetail) error {
	detail.PayrollRowID = rowID
	f.adjustmentDetails = append(f.adjustmentDetails, detail)
	f.operationLog = append(f.operationLog, "adjustment_detail")
	return nil
}

func (f *fakePayrollRepo) VoidDraftRunsForPeriod(ctx context.Context, start, end time.Time, actorID int64, replacementRunID int64) error {
	f.voidStart = start
	f.voidEnd = end
	f.voidActorID = actorID
	f.replacementRunID = replacementRunID
	f.operationLog = append(f.operationLog, "void_drafts")
	return nil
}

func (f *fakePayrollRepo) PersistDraftPayrollRun(ctx context.Context, run model.PayrollRun) (*model.PayrollRun, error) {
	createdRun, err := f.CreatePayrollRun(ctx, run)
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
		createdRow, err := f.CreatePayrollRow(ctx, row)
		if err != nil {
			return nil, err
		}
		for _, detail := range attendanceDetails {
			if err := f.CreatePayrollAttendanceDetail(ctx, createdRow.PayrollRowID, detail); err != nil {
				return nil, err
			}
			detail.DetailID = int64(900 + len(createdRow.AttendanceDetails))
			detail.PayrollRowID = createdRow.PayrollRowID
			createdRow.AttendanceDetails = append(createdRow.AttendanceDetails, detail)
		}
		for _, detail := range bookingDetails {
			if err := f.CreatePayrollBookingDetail(ctx, createdRow.PayrollRowID, detail); err != nil {
				return nil, err
			}
			detail.DetailID = int64(1000 + len(createdRow.BookingDetails))
			detail.PayrollRowID = createdRow.PayrollRowID
			createdRow.BookingDetails = append(createdRow.BookingDetails, detail)
		}
		for _, detail := range adjustmentDetails {
			if err := f.CreatePayrollAdjustmentDetail(ctx, createdRow.PayrollRowID, detail); err != nil {
				return nil, err
			}
			detail.DetailID = int64(1100 + len(createdRow.AdjustmentDetails))
			detail.PayrollRowID = createdRow.PayrollRowID
			createdRow.AdjustmentDetails = append(createdRow.AdjustmentDetails, detail)
		}
		createdRun.Rows = append(createdRun.Rows, *createdRow)
	}
	actorID := int64(0)
	if run.GeneratedBy != nil {
		actorID = *run.GeneratedBy
	}
	if err := f.VoidDraftRunsForPeriod(ctx, run.PeriodStart, run.PeriodEnd, actorID, createdRun.PayrollRunID); err != nil {
		return nil, err
	}
	return createdRun, nil
}

func (f *fakePayrollRepo) HasActivePayrollCoverage(ctx context.Context, sourceKind string, sourceID int64) (bool, error) {
	return f.coverage[payrollCoverageKey(sourceKind, sourceID)], nil
}

func (f *fakePayrollRepo) ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error) {
	return nil, nil
}

func (f *fakePayrollRepo) GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	if f.runDetail == nil {
		return nil, model.ErrNotFound
	}
	copied := *f.runDetail
	copied.Rows = make([]model.PayrollRow, len(f.runDetail.Rows))
	copy(copied.Rows, f.runDetail.Rows)
	return &copied, nil
}

func (f *fakePayrollRepo) ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error {
	f.approvedRunID = runID
	f.approveActorID = actorID
	if f.runForUpdate != nil {
		f.runForUpdate.Status = model.PayrollRunStatusApproved
	}
	for i := range f.listedRows {
		if f.listedRows[i].Status == model.PayrollRowStatusDraft {
			f.listedRows[i].Status = model.PayrollRowStatusApproved
		}
	}
	return nil
}

func (f *fakePayrollRepo) VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error {
	f.voidedRunID = runID
	f.voidedRunActorID = actorID
	f.voidedRunReason = reason
	if f.runForUpdate != nil {
		f.runForUpdate.Status = model.PayrollRunStatusVoided
	}
	for i := range f.listedRows {
		f.listedRows[i].Status = model.PayrollRowStatusVoided
	}
	return nil
}

func (f *fakePayrollRepo) GetPayrollRunForUpdate(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	if f.runForUpdate == nil {
		return nil, model.ErrNotFound
	}
	copied := *f.runForUpdate
	return &copied, nil
}

func (f *fakePayrollRepo) ListPayrollRows(ctx context.Context, runID int64) ([]model.PayrollRow, error) {
	rows := make([]model.PayrollRow, len(f.listedRows))
	copy(rows, f.listedRows)
	return rows, nil
}

func (f *fakePayrollRepo) MarkPayrollRowPaid(ctx context.Context, rowID int64, paidBy int64, method, reference, notes string, ledgerEntryID int64) error {
	f.paidRowID = rowID
	f.paidBy = paidBy
	f.paidMethod = method
	f.paidReference = reference
	f.paidNotes = notes
	f.paidLedgerEntryID = ledgerEntryID
	for i := range f.listedRows {
		if f.listedRows[i].PayrollRowID == rowID {
			f.listedRows[i].Status = model.PayrollRowStatusPaid
			f.listedRows[i].PaidBy = &paidBy
			f.listedRows[i].PaymentMethod = model.PayrollPaymentMethod(method)
			f.listedRows[i].PaymentReference = reference
			f.listedRows[i].PaymentNotes = notes
			f.listedRows[i].LedgerEntryID = &ledgerEntryID
		}
	}
	return nil
}

func (f *fakePayrollRepo) RecordPayrollRowPayment(ctx context.Context, runID, rowID, paidBy int64, method, reference, notes string) (*model.PayrollRow, error) {
	f.atomicPaidRunID = runID
	f.atomicPaidRowID = rowID
	f.atomicPaidBy = paidBy
	f.atomicPaidMethod = method
	f.atomicPaidReference = reference
	f.atomicPaidNotes = notes
	ledgerEntryID := int64(909)
	for i := range f.listedRows {
		if f.listedRows[i].PayrollRowID == rowID {
			f.listedRows[i].Status = model.PayrollRowStatusPaid
			f.listedRows[i].PaidBy = &paidBy
			f.listedRows[i].PaymentMethod = model.PayrollPaymentMethod(method)
			f.listedRows[i].PaymentReference = reference
			f.listedRows[i].PaymentNotes = notes
			f.listedRows[i].LedgerEntryID = &ledgerEntryID
			return &f.listedRows[i], nil
		}
	}
	return nil, model.ErrNotFound
}

func (f *fakePayrollRepo) UpdatePayrollRunPaidIfComplete(ctx context.Context, runID int64) error {
	f.runPaidIfCompleteID = runID
	return nil
}

func (f *fakePayrollRepo) CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error) {
	return f.stale, f.staleReasons, nil
}

type fakePayrollLedgerRepo struct {
	payrollRunID int64
	payrollRowID int64
	userID       int64
	role         repository.TargetRole
	amount       float64
	method       string
	reference    string
	recordedBy   int64
	entryID      int64
}

func (f *fakePayrollLedgerRepo) RecordPayrollSettlement(ctx context.Context, payrollRunID, payrollRowID, userID int64, role repository.TargetRole, amount float64, method, reference string, recordedBy int64) (int64, error) {
	f.payrollRunID = payrollRunID
	f.payrollRowID = payrollRowID
	f.userID = userID
	f.role = role
	f.amount = amount
	f.method = method
	f.reference = reference
	f.recordedBy = recordedBy
	if f.entryID == 0 {
		f.entryID = 909
	}
	return f.entryID, nil
}

func payrollCoverageKey(kind string, id int64) string {
	return kind + ":" + strconv.FormatInt(id, 10)
}

func TestPayrollServiceBuildPayrollWorkbookIncludesSummaryAndDetails(t *testing.T) {
	run := payrollExportFixtureRun(t)
	service := NewPayrollService(&fakePayrollRepo{runDetail: run})

	workbook, err := service.BuildPayrollWorkbook(context.Background(), run.PayrollRunID)
	if err != nil {
		t.Fatalf("BuildPayrollWorkbook returned error: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(workbook))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()

	for _, sheet := range []string{"Summary", "Blockers", "Staff Rows", "Attendance Details", "Booking Details", "Adjustments", "Settlements"} {
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatalf("read sheet %s: %v", sheet, err)
		}
		if len(rows) == 0 {
			t.Fatalf("expected sheet %s to contain rows", sheet)
		}
	}
	assertPayrollWorkbookContains(t, f, "Summary", "Payroll Run")
	assertPayrollWorkbookContains(t, f, "Summary", "Ada Rider")
	assertPayrollWorkbookContains(t, f, "Staff Rows", "Bea Therapist")
	assertPayrollWorkbookContains(t, f, "Attendance Details", "attendance")
	assertPayrollWorkbookContains(t, f, "Booking Details", "Deep Tissue")
	assertPayrollWorkbookContains(t, f, "Adjustments", "performance bonus")
	assertPayrollWorkbookContains(t, f, "Blockers", "missing_rate")
	assertPayrollWorkbookContains(t, f, "Settlements", "GCASH-7")
}

func TestPayrollServiceBuildPayrollPayslipPDFIncludesStaffPage(t *testing.T) {
	run := payrollExportFixtureRun(t)
	service := NewPayrollService(&fakePayrollRepo{runDetail: run})

	pdf, err := service.BuildPayrollPayslipPDF(context.Background(), run.PayrollRunID, true)
	if err != nil {
		t.Fatalf("BuildPayrollPayslipPDF returned error: %v", err)
	}
	text := string(pdf)
	for _, expected := range []string{"Kalinga Spa", "Payroll Period", "DRAFT", "Ada Rider", "Rider", "Main Branch", "attendance", "regular minutes", "final pay", "Acknowledgment", "Kalinga Spa Payroll - generated from payroll snapshots"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected PDF bytes to contain %q", expected)
		}
	}
}

func payrollExportFixtureRun(t *testing.T) *model.PayrollRun {
	t.Helper()
	start := mustPayrollDate(t, "2026-05-01")
	end := mustPayrollDate(t, "2026-05-15")
	workDate := mustPayrollDate(t, "2026-05-02")
	timeIn := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
	timeOut := time.Date(2026, 5, 2, 18, 30, 0, 0, time.UTC)
	branchID := int64(3)
	ledgerEntryID := int64(909)
	paidBy := int64(7)
	paidAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	dailyRate := model.PayrollMoneyCents(120000)
	multiplier := 1.25
	return &model.PayrollRun{
		PayrollRunID: 71,
		PeriodStart:  start,
		StartDate:    "2026-05-01",
		PeriodEnd:    end,
		EndDate:      "2026-05-15",
		Status:       model.PayrollRunStatusDraft,
		Rows: []model.PayrollRow{
			{
				PayrollRowID:               81,
				PayrollRunID:               71,
				UserID:                     12,
				Role:                       model.PayrollRoleRider,
				FullNameSnapshot:           "Ada Rider",
				UsualBranchIDSnapshot:      &branchID,
				UsualLocationLabelSnapshot: "Main Branch",
				Status:                     model.PayrollRowStatusPaid,
				RegularMinutes:             480,
				OvertimeMinutes:            90,
				DailyRateCents:             &dailyRate,
				OvertimeMultiplier:         &multiplier,
				GrossCents:                 148125,
				AddAdjustmentsCents:        2500,
				MinusAdjustmentsCents:      1000,
				FinalPayCents:              149625,
				PaidAt:                     &paidAt,
				PaidBy:                     &paidBy,
				PaymentMethod:              model.PayrollPaymentMethodGCash,
				PaymentReference:           "GCASH-7",
				PaymentNotes:               "settled",
				LedgerEntryID:              &ledgerEntryID,
				AttendanceDetails: []model.PayrollAttendanceDetail{{
					AttendanceID:       501,
					WorkDate:           workDate,
					Date:               "2026-05-02",
					TimeInAt:           &timeIn,
					TimeOutAt:          &timeOut,
					WorkedMinutes:      570,
					RegularMinutes:     480,
					OvertimeMinutes:    90,
					DailyRateCents:     &dailyRate,
					OvertimeMultiplier: &multiplier,
					GrossCents:         148125,
				}},
				AdjustmentDetails: []model.PayrollAdjustmentDetail{{
					AdjustmentID:   601,
					AdjustmentDate: workDate,
					Date:           "2026-05-02",
					Type:           model.PayrollAdjustmentTypeAdd,
					Category:       "bonus",
					AmountCents:    2500,
					Reason:         "performance bonus",
				}},
			},
			{
				PayrollRowID:               82,
				PayrollRunID:               71,
				UserID:                     22,
				Role:                       model.PayrollRoleTherapist,
				FullNameSnapshot:           "Bea Therapist",
				UsualLocationLabelSnapshot: "Spa Floor",
				Status:                     model.PayrollRowStatusBlocked,
				GrossCents:                 80000,
				FinalPayCents:              80000,
				BlockerCodes:               []string{"missing_rate"},
				BookingDetails: []model.PayrollBookingDetail{{
					BookingID:              701,
					BusinessDate:           workDate,
					Date:                   "2026-05-02",
					ServiceName:            "Deep Tissue",
					DurationMinutes:        90,
					FinalTotalCents:        200000,
					TherapistEarningsCents: 80000,
				}},
			},
		},
	}
}

func assertPayrollWorkbookContains(t *testing.T, f *excelize.File, sheet string, expected string) {
	t.Helper()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read sheet %s: %v", sheet, err)
	}
	for _, row := range rows {
		for _, cell := range row {
			if cell == expected {
				return
			}
		}
	}
	t.Fatalf("expected sheet %s to contain %q, rows=%#v", sheet, expected, rows)
}

func TestPayrollServiceCreateRateAutoClosesPreviousOpenRate(t *testing.T) {
	effectiveFrom := mustPayrollDate(t, "2026-05-18")
	actorID := int64(7)
	repo := &fakePayrollRepo{
		openRate: &model.StaffCompensationRate{
			RateID:        41,
			UserID:        12,
			Role:          model.PayrollRoleRider,
			EffectiveFrom: mustPayrollDate(t, "2026-05-01"),
		},
	}
	service := NewPayrollService(repo)

	created, err := service.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      effectiveFrom,
		Notes:              "  updated rate  ",
	}, actorID)
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created.RateID != 99 {
		t.Fatalf("expected created rate id, got %#v", created)
	}
	if repo.closeRateID != 0 {
		t.Fatalf("expected service to delegate atomic rate handling to repository, got legacy close id=%d", repo.closeRateID)
	}
	if repo.createRate == nil || repo.createRate.CreatedBy == nil || *repo.createRate.CreatedBy != actorID || repo.createRate.UpdatedBy == nil || *repo.createRate.UpdatedBy != actorID {
		t.Fatalf("expected actor fields on created rate, got %#v", repo.createRate)
	}
	if repo.createRate.Notes != "updated rate" {
		t.Fatalf("expected notes trimmed, got %q", repo.createRate.Notes)
	}
}

func TestPayrollServiceCreateRateDefaultsOvertimeMultiplier(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)

	_, err := service.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:         12,
		Role:           model.PayrollRoleAdmin,
		DailyRateCents: 120000,
		EffectiveFrom:  mustPayrollDate(t, "2026-05-18"),
	}, 7)
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if repo.createRate == nil || repo.createRate.OvertimeMultiplier != 1.25 {
		t.Fatalf("expected overtime multiplier default 1.25, got %#v", repo.createRate)
	}
}

func TestPayrollServiceCreateRateRejectsNegativeOvertimeMultiplier(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)

	_, err := service.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleAdmin,
		DailyRateCents:     120000,
		OvertimeMultiplier: -1,
		EffectiveFrom:      mustPayrollDate(t, "2026-05-18"),
	}, 7)
	if !errors.Is(err, model.ErrInvalidPayrollRate) {
		t.Fatalf("expected invalid rate, got %v", err)
	}
	if repo.createRate != nil {
		t.Fatalf("expected no create, got %#v", repo.createRate)
	}
}

func TestPayrollServiceCreateCompensationRateRejectsInvalidRole(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)

	_, err := service.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleTherapist,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      mustPayrollDate(t, "2026-05-18"),
	}, 7)
	if !errors.Is(err, model.ErrInvalidPayrollRole) {
		t.Fatalf("expected invalid payroll role, got %v", err)
	}
	if repo.createRate != nil {
		t.Fatalf("expected no create, got %#v", repo.createRate)
	}
}

func TestPayrollServiceCreateCompensationRateRejectsLockedPreviousOpenRate(t *testing.T) {
	repo := &fakePayrollRepo{
		openRate:   &model.StaffCompensationRate{RateID: 41, UserID: 12, Role: model.PayrollRoleRider},
		rateLocked: true,
	}
	service := NewPayrollService(repo)

	_, err := service.CreateCompensationRate(context.Background(), model.StaffCompensationRate{
		UserID:             12,
		Role:               model.PayrollRoleRider,
		DailyRateCents:     120000,
		OvertimeMultiplier: 1.25,
		EffectiveFrom:      mustPayrollDate(t, "2026-05-18"),
	}, 7)
	if !errors.Is(err, model.ErrPayrollRateLocked) {
		t.Fatalf("expected locked rate error, got %v", err)
	}
	if repo.closeRateID != 0 || repo.createRate != nil {
		t.Fatalf("expected no close/create, close=%d create=%#v", repo.closeRateID, repo.createRate)
	}
}

func TestPayrollServiceStaffPayrollAdjustmentValidatesAndSetsActor(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)
	actorID := int64(9)

	created, err := service.CreateStaffPayrollAdjustment(context.Background(), model.StaffPayrollAdjustment{
		UserID:         22,
		Role:           model.PayrollRoleAdmin,
		AdjustmentDate: mustPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeAdd,
		Category:       "bonus",
		AmountCents:    5000,
		Reason:         "  Holiday coverage  ",
	}, actorID)
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created.AdjustmentID != 55 {
		t.Fatalf("expected created adjustment id, got %#v", created)
	}
	if repo.createdAdjustment == nil || repo.createdAdjustment.Reason != "Holiday coverage" {
		t.Fatalf("expected trimmed adjustment, got %#v", repo.createdAdjustment)
	}
	if repo.createdAdjustment.CreatedBy == nil || *repo.createdAdjustment.CreatedBy != actorID || repo.createdAdjustment.UpdatedBy == nil || *repo.createdAdjustment.UpdatedBy != actorID {
		t.Fatalf("expected actor audit on created adjustment, got %#v", repo.createdAdjustment)
	}
}

func TestPayrollServiceStaffPayrollAdjustmentUpdateSetsActor(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)
	actorID := int64(10)

	_, err := service.UpdateStaffPayrollAdjustment(context.Background(), model.StaffPayrollAdjustment{
		AdjustmentID:   44,
		UserID:         22,
		Role:           model.PayrollRoleRider,
		AdjustmentDate: mustPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeMinus,
		Category:       "deduction",
		AmountCents:    500,
		Reason:         "Deduction",
	}, actorID)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if repo.updatedAdjustment == nil || repo.updatedAdjustment.UpdatedBy == nil || *repo.updatedAdjustment.UpdatedBy != actorID {
		t.Fatalf("expected actor audit on updated adjustment, got %#v", repo.updatedAdjustment)
	}
}

func TestPayrollServiceStaffPayrollAdjustmentRejectsInvalidAmountReasonRoleAndLockedUpdate(t *testing.T) {
	service := NewPayrollService(&fakePayrollRepo{})
	base := model.StaffPayrollAdjustment{
		UserID:         22,
		Role:           model.PayrollRoleRider,
		AdjustmentDate: mustPayrollDate(t, "2026-05-17"),
		PeriodStart:    mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:      mustPayrollDate(t, "2026-05-31"),
		Type:           model.PayrollAdjustmentTypeMinus,
		Category:       "deduction",
		AmountCents:    100,
		Reason:         "Reason",
	}

	invalidAmount := base
	invalidAmount.AmountCents = 0
	if _, err := service.CreateStaffPayrollAdjustment(context.Background(), invalidAmount, 1); !errors.Is(err, model.ErrInvalidPayrollAdjustment) {
		t.Fatalf("expected invalid adjustment for amount, got %v", err)
	}

	invalidReason := base
	invalidReason.Reason = " "
	if _, err := service.CreateStaffPayrollAdjustment(context.Background(), invalidReason, 1); !errors.Is(err, model.ErrInvalidPayrollAdjustment) {
		t.Fatalf("expected invalid adjustment for reason, got %v", err)
	}

	invalidRole := base
	invalidRole.Role = model.PayrollRole("super_admin")
	if _, err := service.CreateStaffPayrollAdjustment(context.Background(), invalidRole, 1); !errors.Is(err, model.ErrInvalidPayrollRole) {
		t.Fatalf("expected invalid role, got %v", err)
	}

	invalidCategory := base
	invalidCategory.Category = "salary"
	if _, err := service.CreateStaffPayrollAdjustment(context.Background(), invalidCategory, 1); !errors.Is(err, model.ErrInvalidPayrollAdjustment) {
		t.Fatalf("expected invalid adjustment for category, got %v", err)
	}

	lockedRepo := &fakePayrollRepo{adjustmentLocked: true}
	lockedService := NewPayrollService(lockedRepo)
	base.AdjustmentID = 44
	if _, err := lockedService.UpdateStaffPayrollAdjustment(context.Background(), base, 1); !errors.Is(err, model.ErrPayrollAdjustmentLocked) {
		t.Fatalf("expected locked adjustment, got %v", err)
	}
	if err := lockedService.VoidStaffPayrollAdjustment(context.Background(), 44, 1); !errors.Is(err, model.ErrPayrollAdjustmentLocked) {
		t.Fatalf("expected locked adjustment on void, got %v", err)
	}
}

func TestPayrollServiceGenerateDraftIncludesBlockedAttendanceRows(t *testing.T) {
	periodStart := mustPayrollDate(t, "2026-05-01")
	periodEnd := mustPayrollDate(t, "2026-05-31")
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID:    101,
			UserID:          12,
			Role:            model.PayrollRoleRider,
			FullName:        "Rider One",
			WorkDate:        mustPayrollDate(t, "2026-05-17"),
			SourceUpdatedAt: time.Now(),
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Status != model.PayrollRowStatusBlocked {
		t.Fatalf("expected one blocked row, got %#v", run.Rows)
	}
	if !hasBlocker(run.Rows[0].BlockerCodes, "incomplete_attendance") {
		t.Fatalf("expected incomplete attendance blocker, got %#v", run.Rows[0].BlockerCodes)
	}
	if len(repo.attendanceDetails) != 1 || repo.attendanceDetails[0].AttendanceID != 101 {
		t.Fatalf("expected attendance snapshot, got %#v", repo.attendanceDetails)
	}
}

func TestPayrollServiceGenerateDraftCalculatesRiderDailyRatePay(t *testing.T) {
	timeIn := time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(9 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 201,
			UserID:       22,
			Role:         model.PayrollRoleRider,
			FullName:     "Rider Two",
			WorkDate:     mustPayrollDate(t, "2026-05-17"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
		effectiveRates: map[int64]*model.StaffCompensationRate{
			22: {UserID: 22, Role: model.PayrollRoleRider, DailyRateCents: 80000, OvertimeMultiplier: 1.25},
		},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 {
		t.Fatalf("expected one row, got %#v", run.Rows)
	}
	want := CalculateDailyRatePay(80000, 1.25, 540)
	if int64(run.Rows[0].GrossCents) != want.GrossCents || run.Rows[0].RegularMinutes != want.RegularMinutes || run.Rows[0].OvertimeMinutes != want.OvertimeMinutes {
		t.Fatalf("expected daily rate pay %#v, got row %#v", want, run.Rows[0])
	}
}

func TestPayrollServiceGenerateDraftCalculatesAdminDailyRatePay(t *testing.T) {
	timeIn := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(4 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 301,
			UserID:       32,
			Role:         model.PayrollRoleAdmin,
			FullName:     "Admin One",
			WorkDate:     mustPayrollDate(t, "2026-05-18"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
		effectiveRates: map[int64]*model.StaffCompensationRate{
			32: {UserID: 32, Role: model.PayrollRoleAdmin, DailyRateCents: 100000, OvertimeMultiplier: 1.25},
		},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	want := CalculateDailyRatePay(100000, 1.25, 240)
	if len(run.Rows) != 1 || int64(run.Rows[0].GrossCents) != want.GrossCents {
		t.Fatalf("expected admin gross %d, got %#v", want.GrossCents, run.Rows)
	}
}

func TestPayrollServiceGenerateDraftBlocksMissingRate(t *testing.T) {
	timeIn := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(8 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 351,
			UserID:       35,
			Role:         model.PayrollRoleRider,
			FullName:     "Rider Missing Rate",
			WorkDate:     mustPayrollDate(t, "2026-05-18"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Status != model.PayrollRowStatusBlocked || !hasBlocker(run.Rows[0].BlockerCodes, "missing_rate") {
		t.Fatalf("expected missing_rate blocked row, got %#v", run.Rows)
	}
	if run.Rows[0].GrossCents != 0 {
		t.Fatalf("expected missing rate row to have zero gross, got %#v", run.Rows[0])
	}
}

func TestPayrollServiceGenerateDraftBlocksOverlappingAttendanceSource(t *testing.T) {
	timeIn := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(8 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 361,
			UserID:       36,
			Role:         model.PayrollRoleAdmin,
			FullName:     "Admin Overlap",
			WorkDate:     mustPayrollDate(t, "2026-05-18"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
		effectiveRates: map[int64]*model.StaffCompensationRate{
			36: {UserID: 36, Role: model.PayrollRoleAdmin, DailyRateCents: 100000, OvertimeMultiplier: 1.25},
		},
		coverage: map[string]bool{
			payrollCoverageKey("attendance", 361): true,
		},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Status != model.PayrollRowStatusBlocked || !hasBlocker(run.Rows[0].BlockerCodes, "overlapping_payroll_source") {
		t.Fatalf("expected overlapping payroll source blocker, got %#v", run.Rows)
	}
}

func TestPayrollServiceGenerateDraftBlocksInvalidTherapistBookingSource(t *testing.T) {
	repo := &fakePayrollRepo{
		bookingSources: []repository.PayrollBookingSource{{
			BookingID:              371,
			UserID:                 37,
			Role:                   model.PayrollRoleTherapist,
			FullName:               "Therapist Invalid",
			BusinessDate:           mustPayrollDate(t, "2026-05-18"),
			Status:                 model.BookingStatusCancelled,
			ServiceName:            "Massage",
			DurationMinutes:        90,
			FinalTotalCents:        200000,
			TherapistEarningsCents: 90000,
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Status != model.PayrollRowStatusBlocked || !hasBlocker(run.Rows[0].BlockerCodes, "invalid_source_state") {
		t.Fatalf("expected invalid source state blocker, got %#v", run.Rows)
	}
	if run.Rows[0].GrossCents != 0 {
		t.Fatalf("expected invalid booking to not add gross, got %#v", run.Rows[0])
	}
	if len(repo.bookingDetails) != 1 || repo.bookingDetails[0].BookingID != 371 {
		t.Fatalf("expected invalid booking snapshot, got %#v", repo.bookingDetails)
	}
}

func TestPayrollServiceGenerateDraftSnapshotsTherapistCommissionRows(t *testing.T) {
	repo := &fakePayrollRepo{
		bookingSources: []repository.PayrollBookingSource{{
			BookingID:              401,
			UserID:                 42,
			Role:                   model.PayrollRoleTherapist,
			FullName:               "Therapist One",
			BusinessDate:           mustPayrollDate(t, "2026-05-18"),
			Status:                 model.BookingStatusCompleted,
			ServiceName:            "Massage",
			DurationMinutes:        90,
			FinalTotalCents:        200000,
			TherapistEarningsCents: 90000,
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Role != model.PayrollRoleTherapist || run.Rows[0].GrossCents != 90000 {
		t.Fatalf("expected therapist commission row, got %#v", run.Rows)
	}
	if len(repo.bookingDetails) != 1 || repo.bookingDetails[0].BookingID != 401 || repo.bookingDetails[0].TherapistEarningsCents != 90000 {
		t.Fatalf("expected booking snapshot, got %#v", repo.bookingDetails)
	}
}

func TestPayrollServiceGenerateDraftSnapshotsAdjustmentsOnce(t *testing.T) {
	repo := &fakePayrollRepo{
		adjustmentSources: []repository.PayrollAdjustmentSource{
			{
				AdjustmentID:   501,
				UserID:         52,
				Role:           model.PayrollRoleAdmin,
				FullName:       "Admin Two",
				AdjustmentDate: mustPayrollDate(t, "2026-05-18"),
				Type:           model.PayrollAdjustmentTypeAdd,
				Category:       "bonus",
				AmountCents:    1000,
				Reason:         "Bonus",
			},
			{
				AdjustmentID:   501,
				UserID:         52,
				Role:           model.PayrollRoleAdmin,
				FullName:       "Admin Two",
				AdjustmentDate: mustPayrollDate(t, "2026-05-18"),
				Type:           model.PayrollAdjustmentTypeAdd,
				Category:       "bonus",
				AmountCents:    1000,
				Reason:         "Bonus",
			},
		},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].AddAdjustmentsCents != 1000 || run.Rows[0].FinalPayCents != 1000 {
		t.Fatalf("expected single adjustment application, got %#v", run.Rows)
	}
	if len(repo.adjustmentDetails) != 1 {
		t.Fatalf("expected one adjustment snapshot, got %#v", repo.adjustmentDetails)
	}
}

func TestPayrollServiceGenerateDraftBlocksNegativeFinalPay(t *testing.T) {
	repo := &fakePayrollRepo{
		adjustmentSources: []repository.PayrollAdjustmentSource{{
			AdjustmentID:   551,
			UserID:         55,
			Role:           model.PayrollRoleAdmin,
			FullName:       "Admin Negative",
			AdjustmentDate: mustPayrollDate(t, "2026-05-18"),
			Type:           model.PayrollAdjustmentTypeMinus,
			Category:       "deduction",
			AmountCents:    1000,
			Reason:         "Deduction",
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 1 || run.Rows[0].Status != model.PayrollRowStatusBlocked || !hasBlocker(run.Rows[0].BlockerCodes, "negative_final_pay") {
		t.Fatalf("expected negative final pay blocker, got %#v", run.Rows)
	}
	if run.Rows[0].FinalPayCents != -1000 {
		t.Fatalf("expected negative final pay retained for review, got %#v", run.Rows[0])
	}
}

func TestPayrollServiceGenerateDraftVoidsOldDraftRunsAfterRowsAndDetailsCreated(t *testing.T) {
	repo := &fakePayrollRepo{
		adjustmentSources: []repository.PayrollAdjustmentSource{{
			AdjustmentID:   561,
			UserID:         56,
			Role:           model.PayrollRoleAdmin,
			FullName:       "Admin Void Order",
			AdjustmentDate: mustPayrollDate(t, "2026-05-18"),
			Type:           model.PayrollAdjustmentTypeAdd,
			Category:       "bonus",
			AmountCents:    1000,
			Reason:         "Bonus",
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	wantLog := []string{"create_run", "create_row", "adjustment_detail", "void_drafts"}
	if !sameStrings(repo.operationLog, wantLog) {
		t.Fatalf("expected void after rows/details, got log %#v", repo.operationLog)
	}
	if repo.replacementRunID != run.PayrollRunID || repo.voidActorID != 7 {
		t.Fatalf("expected void metadata to reference new run and actor, got run=%d actor=%d", repo.replacementRunID, repo.voidActorID)
	}
}

func TestPayrollServiceGenerateDraftExcludesZeroActivityStaff(t *testing.T) {
	repo := &fakePayrollRepo{}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 0 || len(repo.createdRows) != 0 {
		t.Fatalf("expected no rows, got run=%#v created=%#v", run.Rows, repo.createdRows)
	}
}

func TestPayrollServiceGenerateDraftExcludesTherapistAttendanceWithoutPayActivity(t *testing.T) {
	timeIn := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(8 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 571,
			UserID:       57,
			Role:         model.PayrollRoleTherapist,
			FullName:     "Therapist Attendance Only",
			WorkDate:     mustPayrollDate(t, "2026-05-18"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 0 || len(repo.createdRows) != 0 || len(repo.attendanceDetails) != 0 {
		t.Fatalf("expected therapist attendance alone to create no payroll activity, rows=%#v created=%#v details=%#v", run.Rows, repo.createdRows, repo.attendanceDetails)
	}
}

func TestPayrollServiceGenerateDraftExcludesSuperAdmin(t *testing.T) {
	timeIn := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	timeOut := timeIn.Add(8 * time.Hour)
	repo := &fakePayrollRepo{
		attendanceSources: []repository.PayrollAttendanceSource{{
			AttendanceID: 601,
			UserID:       62,
			Role:         model.PayrollRole(model.RoleSuperAdmin),
			FullName:     "Owner",
			WorkDate:     mustPayrollDate(t, "2026-05-18"),
			TimeInAt:     &timeIn,
			TimeOutAt:    &timeOut,
		}},
	}
	service := NewPayrollService(repo)

	run, err := service.GenerateDraftPayrollRun(context.Background(), model.PayrollGenerationFilter{
		PeriodStart: mustPayrollDate(t, "2026-05-01"),
		PeriodEnd:   mustPayrollDate(t, "2026-05-31"),
		GeneratedBy: 7,
	})
	if err != nil {
		t.Fatalf("expected generate to succeed, got %v", err)
	}
	if len(run.Rows) != 0 {
		t.Fatalf("expected super admin excluded, got %#v", run.Rows)
	}
}

func TestPayrollServiceApproveBlocksRowsWithBlockers(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusDraft},
		listedRows: []model.PayrollRow{{
			PayrollRowID:  81,
			PayrollRunID:  71,
			Status:        model.PayrollRowStatusBlocked,
			BlockerCodes:  []string{"missing_rate"},
			FinalPayCents: 1000,
		}},
	}
	service := NewPayrollService(repo)

	err := service.ApprovePayrollRun(context.Background(), 71, 7)
	if !errors.Is(err, model.ErrPayrollRunHasBlockers) {
		t.Fatalf("expected blockers error, got %v", err)
	}
	if repo.approvedRunID != 0 {
		t.Fatalf("expected approve not to be persisted, got run %d", repo.approvedRunID)
	}
}

func TestPayrollServiceApproveLocksCleanDraft(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusDraft},
		listedRows: []model.PayrollRow{{
			PayrollRowID:  81,
			PayrollRunID:  71,
			Status:        model.PayrollRowStatusDraft,
			FinalPayCents: 1000,
		}},
	}
	service := NewPayrollService(repo)

	if err := service.ApprovePayrollRun(context.Background(), 71, 7); err != nil {
		t.Fatalf("expected approve to succeed, got %v", err)
	}
	if repo.approvedRunID != 71 || repo.approveActorID != 7 {
		t.Fatalf("expected approve metadata, got run=%d actor=%d", repo.approvedRunID, repo.approveActorID)
	}
	if repo.listedRows[0].Status != model.PayrollRowStatusApproved {
		t.Fatalf("expected clean row approved, got %#v", repo.listedRows[0])
	}
}

func TestPayrollServiceVoidApprovedRunUnlocksSources(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusApproved},
		listedRows: []model.PayrollRow{{
			PayrollRowID: 81,
			PayrollRunID: 71,
			Status:       model.PayrollRowStatusApproved,
		}},
	}
	service := NewPayrollService(repo)

	if err := service.VoidPayrollRun(context.Background(), 71, 7, " duplicate "); err != nil {
		t.Fatalf("expected void to succeed, got %v", err)
	}
	if repo.voidedRunID != 71 || repo.voidedRunActorID != 7 || repo.voidedRunReason != "duplicate" {
		t.Fatalf("expected void metadata, got run=%d actor=%d reason=%q", repo.voidedRunID, repo.voidedRunActorID, repo.voidedRunReason)
	}
	if repo.listedRows[0].Status != model.PayrollRowStatusVoided {
		t.Fatalf("expected row voided so sources unlock, got %#v", repo.listedRows[0])
	}
}

func TestPayrollServiceVoidApprovedRunRejectsPaidRows(t *testing.T) {
	ledgerEntryID := int64(909)
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusApproved},
		listedRows: []model.PayrollRow{{
			PayrollRowID:  81,
			PayrollRunID:  71,
			Status:        model.PayrollRowStatusPaid,
			LedgerEntryID: &ledgerEntryID,
		}},
	}
	service := NewPayrollService(repo)

	err := service.VoidPayrollRun(context.Background(), 71, 7, "duplicate")
	if !errors.Is(err, model.ErrPayrollRunImmutable) {
		t.Fatalf("expected immutable error for paid row, got %v", err)
	}
	if repo.voidedRunID != 0 {
		t.Fatalf("expected no void persistence, got run %d", repo.voidedRunID)
	}
}

func TestPayrollServiceMarkRowPaidCreatesSettlementEntry(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusApproved},
		listedRows: []model.PayrollRow{{
			PayrollRowID:     81,
			PayrollRunID:     71,
			UserID:           22,
			Role:             model.PayrollRoleAdmin,
			FullNameSnapshot: "Admin One",
			Status:           model.PayrollRowStatusApproved,
			FinalPayCents:    12345,
		}},
	}
	service := NewPayrollService(repo)

	row, err := service.MarkPayrollRowPaid(context.Background(), 71, 81, 7, model.PayrollPaymentRequest{
		PaymentMethod:    model.PayrollPaymentMethodCash,
		PaymentReference: " CASH-1 ",
		PaymentNotes:     " Paid in office ",
	})
	if err != nil {
		t.Fatalf("expected mark paid to succeed, got %v", err)
	}
	if repo.atomicPaidRunID != 71 || repo.atomicPaidRowID != 81 || repo.atomicPaidBy != 7 {
		t.Fatalf("expected atomic payment target, got repo=%#v", repo)
	}
	if repo.atomicPaidMethod != string(model.PayrollPaymentMethodCash) || repo.atomicPaidReference != "CASH-1" || repo.atomicPaidNotes != "Paid in office" {
		t.Fatalf("expected trimmed atomic payment details, got repo=%#v", repo)
	}
	if repo.paidRowID != 0 || repo.runPaidIfCompleteID != 0 {
		t.Fatalf("expected service not to use split paid path, repo=%#v", repo)
	}
	if row == nil || row.Status != model.PayrollRowStatusPaid || row.LedgerEntryID == nil || *row.LedgerEntryID != 909 {
		t.Fatalf("expected returned paid row with ledger id, got %#v", row)
	}
}

func TestPayrollServiceMarkRowPaidRequiresPaymentMethod(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusApproved},
		listedRows: []model.PayrollRow{{
			PayrollRowID:  81,
			PayrollRunID:  71,
			UserID:        22,
			Role:          model.PayrollRoleRider,
			Status:        model.PayrollRowStatusApproved,
			FinalPayCents: 1000,
		}},
	}
	service := NewPayrollService(repo)

	_, err := service.MarkPayrollRowPaid(context.Background(), 71, 81, 7, model.PayrollPaymentRequest{})
	if !errors.Is(err, model.ErrPayrollPaymentMethodRequired) {
		t.Fatalf("expected payment method error, got %v", err)
	}
	if repo.atomicPaidRowID != 0 || repo.paidRowID != 0 {
		t.Fatalf("expected no paid mutation, repo=%#v", repo)
	}
}

func TestPayrollServiceMarkRowPaidRejectsInvalidPaymentMethod(t *testing.T) {
	repo := &fakePayrollRepo{
		runForUpdate: &model.PayrollRun{PayrollRunID: 71, Status: model.PayrollRunStatusApproved},
		listedRows: []model.PayrollRow{{
			PayrollRowID:  81,
			PayrollRunID:  71,
			UserID:        22,
			Role:          model.PayrollRoleRider,
			Status:        model.PayrollRowStatusApproved,
			FinalPayCents: 1000,
		}},
	}
	service := NewPayrollService(repo)

	_, err := service.MarkPayrollRowPaid(context.Background(), 71, 81, 7, model.PayrollPaymentRequest{
		PaymentMethod: model.PayrollPaymentMethod("cheque"),
	})
	if !errors.Is(err, model.ErrInvalidPayrollPaymentMethod) {
		t.Fatalf("expected invalid payment method, got %v", err)
	}
	if repo.atomicPaidRowID != 0 {
		t.Fatalf("expected no paid mutation, repo=%#v", repo)
	}
}

func TestPayrollServiceRunIsStaleWhenSourceUpdatedAfterGeneration(t *testing.T) {
	repo := &fakePayrollRepo{
		stale:        true,
		staleReasons: []string{"attendance_source_updated"},
	}
	service := NewPayrollService(repo)

	stale, reasons, err := service.CheckPayrollRunStaleness(context.Background(), 71)
	if err != nil {
		t.Fatalf("expected staleness check to succeed, got %v", err)
	}
	if !stale || !sameStrings(reasons, []string{"attendance_source_updated"}) {
		t.Fatalf("expected stale attendance reason, stale=%v reasons=%#v", stale, reasons)
	}
}

func TestPayrollServiceRunIsStaleWhenNewSourceAppears(t *testing.T) {
	repo := &fakePayrollRepo{
		stale:        true,
		staleReasons: []string{"new_attendance_source", "new_booking_source", "new_adjustment_source"},
	}
	service := NewPayrollService(repo)

	stale, reasons, err := service.CheckPayrollRunStaleness(context.Background(), 71)
	if err != nil {
		t.Fatalf("expected staleness check to succeed, got %v", err)
	}
	if !stale || !sameStrings(reasons, []string{"new_attendance_source", "new_booking_source", "new_adjustment_source"}) {
		t.Fatalf("expected new source reasons, stale=%v reasons=%#v", stale, reasons)
	}
}

func hasBlocker(blockers []string, want string) bool {
	for _, blocker := range blockers {
		if blocker == want {
			return true
		}
	}
	return false
}

func sameStrings(got []string, want []string) bool {
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

func mustPayrollDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}
