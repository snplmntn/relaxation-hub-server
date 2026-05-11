package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/xuri/excelize/v2"
)

type ReportExportService interface {
	BuildDailySalesReport(ctx context.Context, businessDate time.Time) (*model.DailySalesReport, error)
	UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error)
	BuildDailySalesWorkbook(report model.DailySalesReport) ([]byte, error)
	ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error)
	CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error)
	VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error
	BuildSalaryReport(ctx context.Context, filter model.SalaryReportFilter) (*model.SalaryReport, error)
	BuildSalaryWorkbook(report model.SalaryReport) ([]byte, error)
}

type reportExportService struct {
	repo repository.ReportExportRepository
}

func NewReportExportService(repo repository.ReportExportRepository) ReportExportService {
	return &reportExportService{repo: repo}
}

func (s *reportExportService) BuildDailySalesReport(ctx context.Context, businessDate time.Time) (*model.DailySalesReport, error) {
	roster, err := s.repo.ListActiveBranchTherapists(ctx)
	if err != nil {
		return nil, err
	}
	bookingRows, err := s.repo.ListDailySalesBookingRows(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	warnings, err := s.repo.CountDailySalesCompletedBookingsMissingActualEnd(ctx, businessDate)
	if err != nil {
		return nil, err
	}

	salesByTherapist := make(map[int64][]model.ReportDailySalesBookingRow, len(bookingRows))
	for _, row := range bookingRows {
		salesByTherapist[row.TherapistID] = append(salesByTherapist[row.TherapistID], row)
	}

	branchIndexes := make(map[int64]int)
	report := model.DailySalesReport{
		BusinessDate: businessDate,
		Date:         businessDate.Format("2006-01-02"),
		Warnings:     model.ReportWarningCounts{CompletedBookingsMissingActualEnd: warnings},
	}

	for _, rosterRow := range roster {
		branchIndex, ok := branchIndexes[rosterRow.BranchID]
		if !ok {
			branchIndex = len(report.Branches)
			branchIndexes[rosterRow.BranchID] = branchIndex
			report.Branches = append(report.Branches, model.DailySalesBranchSection{BranchID: rosterRow.BranchID, BranchName: rosterRow.BranchName})
		}

		therapistRow := model.DailySalesTherapistRow{TherapistID: rosterRow.TherapistID, TherapistName: rosterRow.TherapistName}
		for _, salesRow := range salesByTherapist[rosterRow.TherapistID] {
			sumDailySalesRow(&therapistRow, salesRow)
		}

		branch := &report.Branches[branchIndex]
		appendDailySalesTherapist(branch, therapistRow)
	}

	for therapistID, rows := range salesByTherapist {
		if dailySalesTherapistListed(report.Branches, therapistID) || len(rows) == 0 {
			continue
		}
		first := rows[0]
		branchID := first.BranchID
		branchName := first.BranchName
		if branchName == "" {
			branchName = "Unassigned"
		}
		branchIndex, ok := branchIndexes[branchID]
		if !ok {
			branchIndex = len(report.Branches)
			branchIndexes[branchID] = branchIndex
			report.Branches = append(report.Branches, model.DailySalesBranchSection{BranchID: branchID, BranchName: branchName})
		}
		therapistName := first.TherapistName
		if therapistName == "" {
			therapistName = "Unknown Therapist"
		}
		therapistRow := model.DailySalesTherapistRow{TherapistID: therapistID, TherapistName: therapistName}
		for _, row := range rows {
			sumDailySalesRow(&therapistRow, row)
		}
		appendDailySalesTherapist(&report.Branches[branchIndex], therapistRow)
	}

	for i := range report.Branches {
		remittance, err := s.repo.GetDailySalesRemittance(ctx, businessDate, report.Branches[i].BranchID)
		if err != nil {
			return nil, err
		}
		if remittance == nil {
			remittance = &model.DailySalesRemittance{BusinessDate: businessDate, Date: businessDate.Format("2006-01-02"), BranchID: report.Branches[i].BranchID}
		}
		remittance.MustBeZero = CalculateDailySalesMustBeZero(report.Branches[i].Totals.CashSales, *remittance)
		remittance.Date = businessDate.Format("2006-01-02")
		report.Branches[i].Remittance = *remittance
	}

	return &report, nil
}

func (s *reportExportService) UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error) {
	stored, err := s.repo.UpsertDailySalesRemittance(ctx, remittance)
	if err != nil {
		return nil, err
	}
	report, err := s.BuildDailySalesReport(ctx, remittance.BusinessDate)
	if err != nil {
		stored.MustBeZero = CalculateDailySalesMustBeZero(0, *stored)
		return stored, nil
	}
	for _, branch := range report.Branches {
		if branch.BranchID == remittance.BranchID {
			stored.MustBeZero = CalculateDailySalesMustBeZero(branch.Totals.CashSales, *stored)
			break
		}
	}
	return stored, nil
}

func NormalizePaymentBucket(paymentMethod string) string {
	method := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(paymentMethod, "-", "_")))
	method = strings.ReplaceAll(method, " ", "_")
	switch method {
	case "cash":
		return "cash"
	case "gcash", "g_cash":
		return "gcash"
	case "spa_remit", "spa":
		return "spa_remit"
	default:
		return "other"
	}
}

func sumDailySalesRow(therapistRow *model.DailySalesTherapistRow, salesRow model.ReportDailySalesBookingRow) {
	switch NormalizePaymentBucket(salesRow.PaymentMethod) {
	case "cash":
		therapistRow.CashSales += salesRow.TotalSales
	case "gcash":
		therapistRow.GCashSales += salesRow.TotalSales
	case "spa_remit":
		therapistRow.SpaRemitSales += salesRow.TotalSales
	default:
		therapistRow.OtherSales += salesRow.TotalSales
	}
	therapistRow.TotalSales += salesRow.TotalSales
	therapistRow.TotalHours += salesRow.TotalHours
	therapistRow.BookingCount += salesRow.BookingCount
}

func appendDailySalesTherapist(branch *model.DailySalesBranchSection, therapistRow model.DailySalesTherapistRow) {
	branch.Therapists = append(branch.Therapists, therapistRow)
	branch.Totals.CashSales += therapistRow.CashSales
	branch.Totals.GCashSales += therapistRow.GCashSales
	branch.Totals.SpaRemitSales += therapistRow.SpaRemitSales
	branch.Totals.OtherSales += therapistRow.OtherSales
	branch.Totals.TotalSales += therapistRow.TotalSales
	branch.Totals.TotalHours += therapistRow.TotalHours
	branch.Totals.BookingCount += therapistRow.BookingCount
}

func dailySalesTherapistListed(branches []model.DailySalesBranchSection, therapistID int64) bool {
	for _, branch := range branches {
		for _, therapist := range branch.Therapists {
			if therapist.TherapistID == therapistID {
				return true
			}
		}
	}
	return false
}

func CalculateDailySalesMustBeZero(totalCashSales float64, remittance model.DailySalesRemittance) float64 {
	denominationTotal := float64(remittance.Bill1000*1000 + remittance.Bill500*500 + remittance.Bill200*200 + remittance.Bill100*100 + remittance.Bill50*50 + remittance.Bill20*20 + remittance.Bill10*10 + remittance.Bill5*5 + remittance.Bill1)
	actualCash := denominationTotal
	if denominationTotal == 0 {
		actualCash = remittance.ActualRemitted
	}
	expectedCash := totalCashSales + remittance.ClientFundsAdded + remittance.OthersAdded - remittance.ClientFundsUsed - remittance.RemittedToMark - remittance.OtherRemittedAmount - remittance.OthersDeducted - remittance.TipsTotal
	return actualCash - expectedCash
}

func (s *reportExportService) ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error) {
	return s.repo.ListPayrollAdjustments(ctx, filter)
}

func (s *reportExportService) CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	return s.repo.CreatePayrollAdjustment(ctx, adjustment)
}

func (s *reportExportService) UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	return s.repo.UpdatePayrollAdjustment(ctx, adjustment)
}

func (s *reportExportService) VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	return s.repo.VoidPayrollAdjustment(ctx, adjustmentID, actorID)
}

func (s *reportExportService) BuildSalaryReport(ctx context.Context, filter model.SalaryReportFilter) (*model.SalaryReport, error) {
	bookingRows, err := s.repo.ListSalaryBookingRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.repo.ListPayrollAdjustments(ctx, model.PayrollAdjustmentFilter{StartDate: filter.StartDate, EndDate: filter.EndDate, TherapistID: filter.TherapistID})
	if err != nil {
		return nil, err
	}
	warnings, err := s.repo.CountSalaryCompletedBookingsMissingActualEnd(ctx, filter.StartDate, filter.EndDate)
	if err != nil {
		return nil, err
	}

	indexByTherapist := make(map[int64]int)
	report := model.SalaryReport{StartDate: filter.StartDate, EndDate: filter.EndDate, Start: filter.StartDate.Format("2006-01-02"), End: filter.EndDate.Format("2006-01-02"), Warnings: model.ReportWarningCounts{CompletedBookingsMissingActualEnd: warnings}}

	for _, row := range bookingRows {
		idx, ok := indexByTherapist[row.TherapistID]
		if !ok {
			idx = len(report.Therapists)
			indexByTherapist[row.TherapistID] = idx
			report.Therapists = append(report.Therapists, model.SalaryTherapistSummary{TherapistID: row.TherapistID, TherapistName: row.TherapistName})
		}
		summary := &report.Therapists[idx]
		summary.Bookings = append(summary.Bookings, row)
		summary.TotalHours += float64(row.DurationMinutes) / 60.0
		summary.GrossSales += row.FinalTotal
		summary.BookingEarnings += row.TherapistEarnings
	}

	for _, adjustment := range adjustments {
		idx, ok := indexByTherapist[adjustment.TherapistID]
		if !ok {
			idx = len(report.Therapists)
			indexByTherapist[adjustment.TherapistID] = idx
			report.Therapists = append(report.Therapists, model.SalaryTherapistSummary{TherapistID: adjustment.TherapistID, TherapistName: adjustment.TherapistName})
		}
		summary := &report.Therapists[idx]
		summary.Adjustments = append(summary.Adjustments, adjustment)
		if adjustment.Type == model.PayrollAdjustmentTypeAdd {
			summary.AddAdjustments += adjustment.Amount
		} else {
			summary.MinusAdjustments += adjustment.Amount
		}
	}

	for i := range report.Therapists {
		report.Therapists[i].FinalSalary = report.Therapists[i].BookingEarnings + report.Therapists[i].AddAdjustments - report.Therapists[i].MinusAdjustments
	}
	return &report, nil
}

func (s *reportExportService) BuildDailySalesWorkbook(report model.DailySalesReport) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Daily Sales"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := setCells(f, sheet, map[string]any{"A1": "Daily Sales Report", "A2": report.Date, "A3": "Warnings", "B3": report.Warnings.CompletedBookingsMissingActualEnd}); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", style); err != nil {
		return nil, err
	}
	row := 4
	for _, branch := range report.Branches {
		branchCell := fmt.Sprintf("A%d", row)
		if err := f.SetCellValue(sheet, branchCell, branch.BranchName); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheet, branchCell, branchCell, style); err != nil {
			return nil, err
		}
		row++
		headers := []string{"Therapist", "Cash", "GCash", "Spa Remit", "Other", "Total", "Hours", "Bookings"}
		for i, header := range headers {
			cell, err := excelize.CoordinatesToCellName(i+1, row)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, header); err != nil {
				return nil, err
			}
		}
		row++
		for _, therapist := range branch.Therapists {
			values := []any{therapist.TherapistName, therapist.CashSales, therapist.GCashSales, therapist.SpaRemitSales, therapist.OtherSales, therapist.TotalSales, therapist.TotalHours, therapist.BookingCount}
			for i, value := range values {
				cell, err := excelize.CoordinatesToCellName(i+1, row)
				if err != nil {
					return nil, err
				}
				if err := f.SetCellValue(sheet, cell, value); err != nil {
					return nil, err
				}
			}
			row++
		}
		if err := writeRow(f, sheet, row, []any{"Totals", branch.Totals.CashSales, branch.Totals.GCashSales, branch.Totals.SpaRemitSales, branch.Totals.OtherSales, branch.Totals.TotalSales, branch.Totals.TotalHours, branch.Totals.BookingCount}); err != nil {
			return nil, err
		}
		row += 2
		remittanceRows := [][]any{
			{"Bill 1000", branch.Remittance.Bill1000, "Bill 500", branch.Remittance.Bill500, "Bill 200", branch.Remittance.Bill200},
			{"Bill 100", branch.Remittance.Bill100, "Bill 50", branch.Remittance.Bill50, "Bill 20", branch.Remittance.Bill20},
			{"Bill 10", branch.Remittance.Bill10, "Bill 5", branch.Remittance.Bill5, "Bill 1", branch.Remittance.Bill1},
			{"Actual Remitted", branch.Remittance.ActualRemitted, "Tips Total", branch.Remittance.TipsTotal},
			{"Client Funds Used", branch.Remittance.ClientFundsUsed, "Client Funds Added", branch.Remittance.ClientFundsAdded},
			{"Remitted To Mark", branch.Remittance.RemittedToMark, "Other Remitted", branch.Remittance.OtherRemittedAmount},
			{"Remitted To", branch.Remittance.RemittedTo},
			{"Others Deducted", branch.Remittance.OthersDeducted, "Others Added", branch.Remittance.OthersAdded},
			{"Notes", branch.Remittance.Notes},
			{"Must be ZERO", branch.Remittance.MustBeZero},
		}
		for _, values := range remittanceRows {
			if err := writeRow(f, sheet, row, values); err != nil {
				return nil, err
			}
			row++
		}
		row += 2
	}
	return workbookBytes(f)
}

func (s *reportExportService) BuildSalaryWorkbook(report model.SalaryReport) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := setCells(f, "Summary", map[string]any{"A1": "Therapist Salary Report", "A2": report.Start + " to " + report.End, "A3": "Missing actual_end warnings", "B3": report.Warnings.CompletedBookingsMissingActualEnd}); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle("Summary", "A1", "A1", style); err != nil {
		return nil, err
	}
	headers := []string{"Therapist", "Hours", "Gross Sales", "Booking Earnings", "Add", "Minus", "Final Salary"}
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 5)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue("Summary", cell, header); err != nil {
			return nil, err
		}
	}
	usedNames := map[string]struct{}{"Summary": {}}
	for i, therapist := range report.Therapists {
		row := i + 6
		values := []any{therapist.TherapistName, therapist.TotalHours, therapist.GrossSales, therapist.BookingEarnings, therapist.AddAdjustments, therapist.MinusAdjustments, therapist.FinalSalary}
		for col, value := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue("Summary", cell, value); err != nil {
				return nil, err
			}
		}
		sheetName := uniqueSheetName(safeSheetName(therapist.TherapistName), usedNames)
		if _, err := f.NewSheet(sheetName); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetName, "A1", therapist.TherapistName); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheetName, "A1", "A1", style); err != nil {
			return nil, err
		}
		if err := writeRow(f, sheetName, 3, []any{"Bookings"}); err != nil {
			return nil, err
		}
		if err := writeRow(f, sheetName, 4, []any{"Date", "Booking ID", "Service", "Minutes", "Gross Sales", "Therapist Earnings"}); err != nil {
			return nil, err
		}
		detailRow := 5
		for _, booking := range therapist.Bookings {
			values := []any{booking.BusinessDate.Format("2006-01-02"), booking.BookingID, booking.ServiceName, booking.DurationMinutes, booking.FinalTotal, booking.TherapistEarnings}
			for col, value := range values {
				cell, err := excelize.CoordinatesToCellName(col+1, detailRow)
				if err != nil {
					return nil, err
				}
				if err := f.SetCellValue(sheetName, cell, value); err != nil {
					return nil, err
				}
			}
			detailRow++
		}
		detailRow++
		if err := writeRow(f, sheetName, detailRow, []any{"Adjustments"}); err != nil {
			return nil, err
		}
		detailRow++
		if err := writeRow(f, sheetName, detailRow, []any{"Date", "Type", "Category", "Amount", "Cash Movement", "Reason"}); err != nil {
			return nil, err
		}
		for _, adjustment := range therapist.Adjustments {
			detailRow++
			values := []any{adjustment.Date, string(adjustment.Type), string(adjustment.Category), adjustment.Amount, adjustment.CashMovement, adjustment.Reason}
			for col, value := range values {
				cell, err := excelize.CoordinatesToCellName(col+1, detailRow)
				if err != nil {
					return nil, err
				}
				if err := f.SetCellValue(sheetName, cell, value); err != nil {
					return nil, err
				}
			}
		}
		detailRow += 2
		if err := writeRow(f, sheetName, detailRow, []any{"Totals", "Hours", therapist.TotalHours, "Gross Sales", therapist.GrossSales, "Booking Earnings", therapist.BookingEarnings}); err != nil {
			return nil, err
		}
		detailRow++
		if err := writeRow(f, sheetName, detailRow, []any{"Add", therapist.AddAdjustments, "Minus", therapist.MinusAdjustments, "Final Salary", therapist.FinalSalary}); err != nil {
			return nil, err
		}
	}
	return workbookBytes(f)
}

func workbookBytes(f *excelize.File) ([]byte, error) {
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return bytes.Clone(buffer.Bytes()), nil
}

func setCells(f *excelize.File, sheet string, cells map[string]any) error {
	for cell, value := range cells {
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			return err
		}
	}
	return nil
}

func writeRow(f *excelize.File, sheet string, row int, values []any) error {
	for i, value := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			return err
		}
	}
	return nil
}

func safeSheetName(name string) string {
	clean := strings.NewReplacer("/", " ", "\\", " ", "?", " ", "*", " ", "[", " ", "]", " ", ":", " ").Replace(strings.TrimSpace(name))
	if clean == "" {
		return "Therapist"
	}
	if len(clean) > 31 {
		return clean[:31]
	}
	return clean
}

func uniqueSheetName(base string, used map[string]struct{}) string {
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base
	}
	for i := 2; ; i++ {
		suffix := fmt.Sprintf(" (%d)", i)
		limit := 31 - len(suffix)
		candidateBase := base
		if len(candidateBase) > limit {
			candidateBase = candidateBase[:limit]
		}
		candidate := candidateBase + suffix
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}
