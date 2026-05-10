package service

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/xuri/excelize/v2"
)

type fakeReportExportRepository struct {
	roster                 []model.ReportTherapistRosterRow
	dailySales             []model.ReportDailySalesBookingRow
	dailyMissingActualEnd  int
	salaryMissingActualEnd int
	dailyWarningDate       time.Time
	salaryWarningStartDate time.Time
	salaryWarningEndDate   time.Time
	remittances            []model.DailySalesRemittance
	salaryRows             []model.ReportSalaryBookingRow
	adjustments            []model.PayrollAdjustment
}

func (f *fakeReportExportRepository) ListActiveBranchTherapists(ctx context.Context) ([]model.ReportTherapistRosterRow, error) {
	return f.roster, nil
}

func (f *fakeReportExportRepository) ListDailySalesBookingRows(ctx context.Context, businessDate time.Time) ([]model.ReportDailySalesBookingRow, error) {
	return f.dailySales, nil
}

func (f *fakeReportExportRepository) CountDailySalesCompletedBookingsMissingActualEnd(ctx context.Context, businessDate time.Time) (int, error) {
	f.dailyWarningDate = businessDate
	return f.dailyMissingActualEnd, nil
}

func (f *fakeReportExportRepository) CountSalaryCompletedBookingsMissingActualEnd(ctx context.Context, startDate time.Time, endDate time.Time) (int, error) {
	f.salaryWarningStartDate = startDate
	f.salaryWarningEndDate = endDate
	return f.salaryMissingActualEnd, nil
}

func (f *fakeReportExportRepository) GetDailySalesRemittance(ctx context.Context, businessDate time.Time, branchID int64) (*model.DailySalesRemittance, error) {
	for i := range f.remittances {
		remittance := f.remittances[i]
		if remittance.BusinessDate.Equal(businessDate) && remittance.BranchID == branchID {
			return &remittance, nil
		}
	}
	return nil, nil
}

func (f *fakeReportExportRepository) UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error) {
	return &remittance, nil
}

func (f *fakeReportExportRepository) ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error) {
	items := make([]model.PayrollAdjustment, 0, len(f.adjustments))
	for _, adjustment := range f.adjustments {
		if adjustment.PeriodStart.Equal(filter.StartDate) && adjustment.PeriodEnd.Equal(filter.EndDate) {
			items = append(items, adjustment)
		}
	}
	return items, nil
}

func (f *fakeReportExportRepository) CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	adjustment.AdjustmentID = 1
	return &adjustment, nil
}

func (f *fakeReportExportRepository) UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	return &adjustment, nil
}

func (f *fakeReportExportRepository) VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	return nil
}

func (f *fakeReportExportRepository) ListSalaryBookingRows(ctx context.Context, filter model.SalaryReportFilter) ([]model.ReportSalaryBookingRow, error) {
	return f.salaryRows, nil
}

func TestBuildDailySalesReportIncludesZeroSalesTherapistsAndWarnings(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-02-10")
	repo := &fakeReportExportRepository{
		roster: []model.ReportTherapistRosterRow{
			{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Ada"},
			{BranchID: 1, BranchName: "Main", TherapistID: 11, TherapistName: "Bea"},
		},
		dailySales: []model.ReportDailySalesBookingRow{
			{TherapistID: 10, PaymentMethod: "cash", TotalSales: 500, TotalHours: 1.5, BookingCount: 1},
			{TherapistID: 10, PaymentMethod: "g-cash", TotalSales: 700, TotalHours: 2, BookingCount: 1},
		},
		dailyMissingActualEnd: 3,
	}
	service := NewReportExportService(repo)

	report, err := service.BuildDailySalesReport(context.Background(), businessDate)
	if err != nil {
		t.Fatalf("BuildDailySalesReport returned error: %v", err)
	}

	if report.Warnings.CompletedBookingsMissingActualEnd != 3 {
		t.Fatalf("expected missing actual_end warning count 3, got %d", report.Warnings.CompletedBookingsMissingActualEnd)
	}
	if !repo.dailyWarningDate.Equal(businessDate) {
		t.Fatalf("expected warning scoped to business date, got %s", repo.dailyWarningDate.Format("2006-01-02"))
	}
	if len(report.Branches) != 1 || len(report.Branches[0].Therapists) != 2 {
		t.Fatalf("expected one branch with two therapists, got %#v", report.Branches)
	}
	if report.Branches[0].Therapists[0].CashSales != 500 || report.Branches[0].Therapists[0].GCashSales != 700 || report.Branches[0].Therapists[0].TotalSales != 1200 {
		t.Fatalf("unexpected Ada totals: %#v", report.Branches[0].Therapists[0])
	}
	if report.Branches[0].Therapists[1].TotalSales != 0 || report.Branches[0].Therapists[1].BookingCount != 0 {
		t.Fatalf("expected zero-sales therapist row, got %#v", report.Branches[0].Therapists[1])
	}
}

func TestBuildDailySalesReportIncludesCompletedSalesForUnrosteredTherapists(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-02-10")
	repo := &fakeReportExportRepository{
		roster: []model.ReportTherapistRosterRow{{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Ada"}},
		dailySales: []model.ReportDailySalesBookingRow{
			{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Ada", PaymentMethod: "cash", TotalSales: 500, TotalHours: 1, BookingCount: 1},
			{BranchID: 1, BranchName: "Main", TherapistID: 99, TherapistName: "Former Therapist", PaymentMethod: "cash", TotalSales: 900, TotalHours: 2, BookingCount: 1},
		},
	}
	service := NewReportExportService(repo)

	report, err := service.BuildDailySalesReport(context.Background(), businessDate)
	if err != nil {
		t.Fatalf("BuildDailySalesReport returned error: %v", err)
	}
	if len(report.Branches) != 1 || len(report.Branches[0].Therapists) != 2 {
		t.Fatalf("expected active plus unrostered therapist rows, got %#v", report.Branches)
	}
	if report.Branches[0].Totals.CashSales != 1400 || report.Branches[0].Totals.BookingCount != 2 {
		t.Fatalf("expected totals to include unrostered sale, got %#v", report.Branches[0].Totals)
	}
}

func TestCalculateMustBeZeroUsesOtherRemittedAmount(t *testing.T) {
	remittance := model.DailySalesRemittance{
		Bill1000:            1,
		Bill500:             1,
		ClientFundsAdded:    100,
		ClientFundsUsed:     50,
		RemittedToMark:      200,
		OtherRemittedAmount: 150,
		OthersAdded:         25,
		OthersDeducted:      10,
		TipsTotal:           15,
	}

	mustBeZero := CalculateDailySalesMustBeZero(1400, remittance)

	if mustBeZero != 400 {
		t.Fatalf("expected must_be_zero 400, got %.2f", mustBeZero)
	}
}

func TestBuildSalaryReportUsesStoredEarningsAndAdjustments(t *testing.T) {
	startDate, _ := time.Parse("2006-01-02", "2026-02-01")
	endDate, _ := time.Parse("2006-01-02", "2026-02-15")
	therapistID := int64(10)
	repo := &fakeReportExportRepository{
		salaryMissingActualEnd: 2,
		salaryRows: []model.ReportSalaryBookingRow{
			{TherapistID: 10, TherapistName: "Ada", BusinessDate: startDate, ServiceName: "Massage", BookingID: 1, DurationMinutes: 60, FinalTotal: 2000, TherapistEarnings: 800},
			{TherapistID: 10, TherapistName: "Ada", BusinessDate: endDate, ServiceName: "Massage", BookingID: 2, DurationMinutes: 90, FinalTotal: 3000, TherapistEarnings: 1200},
		},
		adjustments: []model.PayrollAdjustment{
			{AdjustmentID: 1, TherapistID: 10, PeriodStart: startDate, PeriodEnd: endDate, Type: model.PayrollAdjustmentTypeAdd, Category: model.PayrollAdjustmentCategoryBenefits, Amount: 250, Reason: "Benefit"},
			{AdjustmentID: 2, TherapistID: 10, PeriodStart: startDate, PeriodEnd: endDate, Type: model.PayrollAdjustmentTypeMinus, Category: model.PayrollAdjustmentCategoryCashAdvance, Amount: 100, Reason: "Advance"},
			{AdjustmentID: 3, TherapistID: 10, PeriodStart: startDate.AddDate(0, 0, -20), PeriodEnd: startDate.AddDate(0, 0, -1), Type: model.PayrollAdjustmentTypeAdd, Category: model.PayrollAdjustmentCategoryBenefits, Amount: 999, Reason: "Wrong period"},
		},
	}
	service := NewReportExportService(repo)

	report, err := service.BuildSalaryReport(context.Background(), model.SalaryReportFilter{StartDate: startDate, EndDate: endDate, TherapistID: &therapistID})
	if err != nil {
		t.Fatalf("BuildSalaryReport returned error: %v", err)
	}

	if len(report.Therapists) != 1 {
		t.Fatalf("expected one therapist, got %d", len(report.Therapists))
	}
	summary := report.Therapists[0]
	if summary.BookingEarnings != 2000 || summary.AddAdjustments != 250 || summary.MinusAdjustments != 100 || summary.FinalSalary != 2150 {
		t.Fatalf("unexpected salary summary: %#v", summary)
	}
	if !repo.salaryWarningStartDate.Equal(startDate) || !repo.salaryWarningEndDate.Equal(endDate) || report.Warnings.CompletedBookingsMissingActualEnd != 2 {
		t.Fatalf("expected warnings scoped to salary period, got %#v", report.Warnings)
	}
}

func TestWorkbookGenerationSmoke(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-02-10")
	report := model.DailySalesReport{
		BusinessDate: businessDate,
		Branches: []model.DailySalesBranchSection{{
			BranchID:   1,
			BranchName: "Main",
			Therapists: []model.DailySalesTherapistRow{{TherapistID: 10, TherapistName: "Ada", CashSales: 500, TotalSales: 500}},
		}},
	}
	service := NewReportExportService(&fakeReportExportRepository{})

	workbook, err := service.BuildDailySalesWorkbook(report)
	if err != nil {
		t.Fatalf("BuildDailySalesWorkbook returned error: %v", err)
	}
	if len(bytes.TrimSpace(workbook)) == 0 {
		t.Fatal("expected non-empty workbook bytes")
	}
	f, err := excelize.OpenReader(bytes.NewReader(workbook))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Daily Sales")
	if err != nil {
		t.Fatalf("read daily sales workbook: %v", err)
	}
	for _, expected := range []string{"Warnings", "Bill 1000", "Actual Remitted", "Tips Total", "Client Funds Used", "Remitted To", "Others Added", "Must be ZERO"} {
		if !workbookRowsContain(rows, expected) {
			t.Fatalf("expected daily sales workbook to include %q", expected)
		}
	}
}

func TestBuildSalaryWorkbookIncludesSummaryDetailsAndUniqueSheets(t *testing.T) {
	startDate, _ := time.Parse("2006-01-02", "2026-02-01")
	report := model.SalaryReport{Start: "2026-02-01", End: "2026-02-15", Therapists: []model.SalaryTherapistSummary{
		{TherapistName: "Very Long Therapist Name That Truncates Same A", Bookings: []model.ReportSalaryBookingRow{{BusinessDate: startDate, BookingID: 1, ServiceName: "Massage", DurationMinutes: 60, FinalTotal: 1000, TherapistEarnings: 400}}, Adjustments: []model.PayrollAdjustment{{Date: "2026-02-03", Type: model.PayrollAdjustmentTypeAdd, Category: model.PayrollAdjustmentCategoryBenefits, Amount: 50, Reason: "Benefit"}}, FinalSalary: 450},
		{TherapistName: "Very Long Therapist Name That Truncates Same B"},
	}}
	service := NewReportExportService(&fakeReportExportRepository{})

	workbook, err := service.BuildSalaryWorkbook(report)
	if err != nil {
		t.Fatalf("BuildSalaryWorkbook returned error: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(workbook))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 3 || sheets[1] == sheets[2] {
		t.Fatalf("expected summary plus unique therapist sheets, got %v", sheets)
	}
	rows, err := f.GetRows(sheets[1])
	if err != nil {
		t.Fatalf("read therapist sheet: %v", err)
	}
	if !workbookRowsContain(rows, "Final Salary") || !workbookRowsContain(rows, "Benefit") {
		t.Fatalf("expected salary workbook details in %#v", rows)
	}
}

func workbookRowsContain(rows [][]string, expected string) bool {
	for _, row := range rows {
		if slices.Contains(row, expected) {
			return true
		}
	}
	return false
}
