package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf/v2"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/xuri/excelize/v2"
)

type PayrollService interface {
	CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate, actorID int64) (*model.StaffCompensationRate, error)
	ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error)
	UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error
	ListStaffPayrollAdjustments(ctx context.Context, filter repository.StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error)
	CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error)
	UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error)
	VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error
	GenerateDraftPayrollRun(ctx context.Context, filter model.PayrollGenerationFilter) (*model.PayrollRun, error)
	ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error)
	GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error)
	ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error
	VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error
	MarkPayrollRowPaid(ctx context.Context, runID, rowID, actorID int64, req model.PayrollPaymentRequest) (*model.PayrollRow, error)
	CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error)
	BuildPayrollWorkbook(ctx context.Context, runID int64) ([]byte, error)
	BuildPayrollPayslipPDF(ctx context.Context, runID int64, draftWatermark bool) ([]byte, error)
}

type payrollService struct {
	repo repository.PayrollRepository
}

func NewPayrollService(repo repository.PayrollRepository) PayrollService {
	return &payrollService{repo: repo}
}

func (s *payrollService) CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate, actorID int64) (*model.StaffCompensationRate, error) {
	if !isPayableRateRole(rate.Role) {
		return nil, model.ErrInvalidPayrollRole
	}
	if rate.UserID <= 0 || rate.DailyRateCents <= 0 || rate.EffectiveFrom.IsZero() {
		return nil, model.ErrInvalidPayrollRate
	}
	if rate.OvertimeMultiplier < 0 {
		return nil, model.ErrInvalidPayrollRate
	}
	if rate.OvertimeMultiplier == 0 {
		rate.OvertimeMultiplier = 1.25
	}
	rate.Notes = strings.TrimSpace(rate.Notes)
	rate.CreatedBy = &actorID
	rate.UpdatedBy = &actorID

	return s.repo.CreateCompensationRateAtomic(ctx, rate)
}

func (s *payrollService) ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error) {
	role = strings.TrimSpace(role)
	if role != "" && !isPayableRateRole(model.PayrollRole(role)) {
		return nil, model.ErrInvalidPayrollRole
	}
	return s.repo.ListCompensationRates(ctx, userID, role)
}

func (s *payrollService) UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error {
	if userID <= 0 {
		return model.ErrInvalidPayrollAdjustment
	}
	return s.repo.UpsertStaffProfile(ctx, userID, branchID, strings.TrimSpace(locationLabel))
}

func (s *payrollService) ListStaffPayrollAdjustments(ctx context.Context, filter repository.StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error) {
	filter.Role = strings.TrimSpace(filter.Role)
	if filter.Role != "" && !isPayrollAdjustmentRole(model.PayrollRole(filter.Role)) {
		return nil, model.ErrInvalidPayrollRole
	}
	return s.repo.ListStaffPayrollAdjustments(ctx, filter)
}

func (s *payrollService) CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error) {
	prepared, err := prepareStaffPayrollAdjustment(adjustment)
	if err != nil {
		return nil, err
	}
	prepared.CreatedBy = &actorID
	prepared.UpdatedBy = &actorID
	return s.repo.CreateStaffPayrollAdjustment(ctx, prepared)
}

func (s *payrollService) UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error) {
	locked, err := s.repo.IsStaffPayrollAdjustmentLocked(ctx, adjustment.AdjustmentID)
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, model.ErrPayrollAdjustmentLocked
	}
	prepared, err := prepareStaffPayrollAdjustment(adjustment)
	if err != nil {
		return nil, err
	}
	prepared.UpdatedBy = &actorID
	return s.repo.UpdateStaffPayrollAdjustment(ctx, prepared)
}

func (s *payrollService) VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	locked, err := s.repo.IsStaffPayrollAdjustmentLocked(ctx, adjustmentID)
	if err != nil {
		return err
	}
	if locked {
		return model.ErrPayrollAdjustmentLocked
	}
	return s.repo.VoidStaffPayrollAdjustment(ctx, adjustmentID, actorID)
}

func (s *payrollService) GenerateDraftPayrollRun(ctx context.Context, filter model.PayrollGenerationFilter) (*model.PayrollRun, error) {
	if filter.GeneratedBy <= 0 || filter.PeriodStart.IsZero() || filter.PeriodEnd.IsZero() || filter.PeriodEnd.Before(filter.PeriodStart) {
		return nil, model.ErrInvalidPayrollAdjustment
	}

	attendanceSources, err := s.repo.ListPayrollAttendanceSources(ctx, filter.PeriodStart, filter.PeriodEnd)
	if err != nil {
		return nil, err
	}
	bookingSources, err := s.repo.ListPayrollTherapistBookingSources(ctx, filter.PeriodStart, filter.PeriodEnd)
	if err != nil {
		return nil, err
	}
	adjustmentSources, err := s.repo.ListPayrollAdjustmentSources(ctx, filter.PeriodStart, filter.PeriodEnd)
	if err != nil {
		return nil, err
	}

	rows := make(map[int64]*payrollDraftRow)
	for _, source := range attendanceSources {
		if !isPayableRateRole(source.Role) {
			continue
		}
		row := payrollDraftRowFor(rows, 0, source.UserID, source.Role, source.FullName, source.UsualBranchIDSnapshot, source.UsualLocationLabelSnapshot)
		detail := model.PayrollAttendanceDetail{
			AttendanceID:    source.AttendanceID,
			WorkDate:        source.WorkDate,
			Date:            source.WorkDate.Format("2006-01-02"),
			TimeInAt:        source.TimeInAt,
			TimeOutAt:       source.TimeOutAt,
			SourceUpdatedAt: source.SourceUpdatedAt,
		}
		if source.TimeInAt != nil && source.TimeOutAt != nil && source.TimeOutAt.After(*source.TimeInAt) {
			detail.WorkedMinutes = int(source.TimeOutAt.Sub(*source.TimeInAt).Minutes())
		}
		if covered, err := s.repo.HasActivePayrollCoverage(ctx, "attendance", source.AttendanceID); err != nil {
			return nil, err
		} else if covered {
			row.addBlocker("overlapping_payroll_source")
		}
		if source.TimeInAt == nil || source.TimeOutAt == nil || detail.WorkedMinutes <= 0 {
			row.addBlocker("incomplete_attendance")
		} else {
			rate, err := s.repo.FindEffectiveRate(ctx, source.UserID, source.WorkDate)
			if errors.Is(err, model.ErrNotFound) {
				row.addBlocker("missing_rate")
			} else if err != nil {
				return nil, err
			} else {
				pay := CalculateDailyRatePay(int64(rate.DailyRateCents), rate.OvertimeMultiplier, detail.WorkedMinutes)
				dailyRate := rate.DailyRateCents
				multiplier := rate.OvertimeMultiplier
				detail.RegularMinutes = pay.RegularMinutes
				detail.OvertimeMinutes = pay.OvertimeMinutes
				detail.DailyRateCents = &dailyRate
				detail.OvertimeMultiplier = &multiplier
				detail.GrossCents = model.PayrollMoneyCents(pay.GrossCents)
				row.RegularMinutes += pay.RegularMinutes
				row.OvertimeMinutes += pay.OvertimeMinutes
				row.GrossCents += model.PayrollMoneyCents(pay.GrossCents)
				row.DailyRateCents = &dailyRate
				row.OvertimeMultiplier = &multiplier
			}
		}
		row.attendanceDetails = append(row.attendanceDetails, detail)
	}

	for _, source := range bookingSources {
		if source.Role == model.PayrollRole(model.RoleSuperAdmin) {
			continue
		}
		row := payrollDraftRowFor(rows, 0, source.UserID, model.PayrollRoleTherapist, source.FullName, source.UsualBranchIDSnapshot, source.UsualLocationLabelSnapshot)
		detail := model.PayrollBookingDetail{
			BookingID:              source.BookingID,
			BusinessDate:           source.BusinessDate,
			Date:                   source.BusinessDate.Format("2006-01-02"),
			ServiceName:            source.ServiceName,
			DurationMinutes:        source.DurationMinutes,
			FinalTotalCents:        source.FinalTotalCents,
			TherapistEarningsCents: source.TherapistEarningsCents,
			SourceUpdatedAt:        source.SourceUpdatedAt,
		}
		if covered, err := s.repo.HasActivePayrollCoverage(ctx, "booking", source.BookingID); err != nil {
			return nil, err
		} else if covered {
			row.addBlocker("overlapping_payroll_source")
		}
		if source.Status != model.BookingStatusCompleted || source.TherapistEarningsCents <= 0 {
			row.addBlocker("invalid_source_state")
		} else {
			row.GrossCents += source.TherapistEarningsCents
		}
		row.bookingDetails = append(row.bookingDetails, detail)
	}

	appliedAdjustments := make(map[int64]struct{})
	for _, source := range adjustmentSources {
		if source.Role == model.PayrollRole(model.RoleSuperAdmin) {
			continue
		}
		if _, exists := appliedAdjustments[source.AdjustmentID]; exists {
			continue
		}
		appliedAdjustments[source.AdjustmentID] = struct{}{}
		row := payrollDraftRowFor(rows, 0, source.UserID, source.Role, source.FullName, source.UsualBranchIDSnapshot, source.UsualLocationLabelSnapshot)
		detail := model.PayrollAdjustmentDetail{
			AdjustmentID:    source.AdjustmentID,
			AdjustmentDate:  source.AdjustmentDate,
			Date:            source.AdjustmentDate.Format("2006-01-02"),
			Type:            source.Type,
			Category:        source.Category,
			AmountCents:     source.AmountCents,
			Reason:          source.Reason,
			SourceUpdatedAt: source.SourceUpdatedAt,
		}
		if covered, err := s.repo.HasActivePayrollCoverage(ctx, "adjustment", source.AdjustmentID); err != nil {
			return nil, err
		} else if covered {
			row.addBlocker("overlapping_payroll_source")
		}
		switch source.Type {
		case model.PayrollAdjustmentTypeAdd:
			row.AddAdjustmentsCents += source.AmountCents
		case model.PayrollAdjustmentTypeMinus:
			row.MinusAdjustmentsCents += source.AmountCents
		default:
			row.addBlocker("invalid_source_state")
		}
		row.adjustmentDetails = append(row.adjustmentDetails, detail)
	}

	drafts := make([]*payrollDraftRow, 0, len(rows))
	for _, row := range rows {
		row.FinalPayCents = row.GrossCents + row.AddAdjustmentsCents - row.MinusAdjustmentsCents
		if row.FinalPayCents < 0 {
			row.addBlocker("negative_final_pay")
		}
		if len(row.blockers) > 0 {
			row.Status = model.PayrollRowStatusBlocked
		} else {
			row.Status = model.PayrollRowStatusDraft
		}
		row.BlockerCodes = row.blockerCodes()
		drafts = append(drafts, row)
	}
	sort.Slice(drafts, func(i, j int) bool {
		if drafts[i].FullNameSnapshot == drafts[j].FullNameSnapshot {
			return drafts[i].UserID < drafts[j].UserID
		}
		return drafts[i].FullNameSnapshot < drafts[j].FullNameSnapshot
	})

	run := model.PayrollRun{
		PeriodStart: filter.PeriodStart,
		StartDate:   filter.PeriodStart.Format("2006-01-02"),
		PeriodEnd:   filter.PeriodEnd,
		EndDate:     filter.PeriodEnd.Format("2006-01-02"),
		Status:      model.PayrollRunStatusDraft,
		GeneratedBy: &filter.GeneratedBy,
		Rows:        make([]model.PayrollRow, 0, len(drafts)),
	}
	for _, draft := range drafts {
		draft.AttendanceDetails = draft.attendanceDetails
		draft.BookingDetails = draft.bookingDetails
		draft.AdjustmentDetails = draft.adjustmentDetails
		run.Rows = append(run.Rows, draft.PayrollRow)
	}

	return s.repo.PersistDraftPayrollRun(ctx, run)
}

func (s *payrollService) ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error) {
	return s.repo.ListPayrollRuns(ctx)
}

func (s *payrollService) GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	if runID <= 0 {
		return nil, model.ErrNotFound
	}
	return s.repo.GetPayrollRun(ctx, runID)
}

func (s *payrollService) ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error {
	if runID <= 0 || actorID <= 0 {
		return model.ErrNotFound
	}
	run, err := s.repo.GetPayrollRunForUpdate(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.PayrollRunStatusDraft {
		return model.ErrPayrollRunImmutable
	}
	rows, err := s.repo.ListPayrollRows(ctx, runID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == model.PayrollRowStatusBlocked || len(row.BlockerCodes) > 0 {
			return model.ErrPayrollRunHasBlockers
		}
		if row.Status != model.PayrollRowStatusDraft {
			return model.ErrPayrollRunImmutable
		}
	}
	return s.repo.ApprovePayrollRun(ctx, runID, actorID)
}

func (s *payrollService) VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error {
	if runID <= 0 || actorID <= 0 {
		return model.ErrNotFound
	}
	run, err := s.repo.GetPayrollRunForUpdate(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.PayrollRunStatusDraft && run.Status != model.PayrollRunStatusApproved {
		return model.ErrPayrollRunImmutable
	}
	rows, err := s.repo.ListPayrollRows(ctx, runID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == model.PayrollRowStatusPaid || row.LedgerEntryID != nil {
			return model.ErrPayrollRunImmutable
		}
	}
	return s.repo.VoidPayrollRun(ctx, runID, actorID, strings.TrimSpace(reason))
}

func (s *payrollService) MarkPayrollRowPaid(ctx context.Context, runID, rowID, actorID int64, req model.PayrollPaymentRequest) (*model.PayrollRow, error) {
	if runID <= 0 || rowID <= 0 || actorID <= 0 {
		return nil, model.ErrNotFound
	}
	method := strings.TrimSpace(string(req.PaymentMethod))
	if method == "" {
		return nil, model.ErrPayrollPaymentMethodRequired
	}
	if !isValidPayrollPaymentMethod(method) {
		return nil, model.ErrInvalidPayrollPaymentMethod
	}
	reference := strings.TrimSpace(req.PaymentReference)
	notes := strings.TrimSpace(req.PaymentNotes)
	return s.repo.RecordPayrollRowPayment(ctx, runID, rowID, actorID, method, reference, notes)
}

func (s *payrollService) CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error) {
	if runID <= 0 {
		return false, nil, model.ErrNotFound
	}
	return s.repo.CheckPayrollRunStaleness(ctx, runID)
}

func (s *payrollService) BuildPayrollWorkbook(ctx context.Context, runID int64) ([]byte, error) {
	run, err := s.GetPayrollRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	for _, sheet := range []string{"Blockers", "Staff Rows", "Attendance Details", "Booking Details", "Adjustments", "Settlements"} {
		if _, err := f.NewSheet(sheet); err != nil {
			return nil, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for _, sheet := range f.GetSheetList() {
		if err := f.SetCellStyle(sheet, "A1", "A1", style); err != nil {
			return nil, err
		}
	}

	if err := writePayrollSummarySheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollStaffRowsSheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollBlockersSheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollAttendanceSheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollBookingSheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollAdjustmentsSheet(f, run); err != nil {
		return nil, err
	}
	if err := writePayrollSettlementsSheet(f, run); err != nil {
		return nil, err
	}
	return workbookBytes(f)
}

func (s *payrollService) BuildPayrollPayslipPDF(ctx context.Context, runID int64, draftWatermark bool) ([]byte, error) {
	run, err := s.GetPayrollRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 16)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(0, 6, fmt.Sprintf("Relaxation Hub Payroll - generated from payroll snapshots | Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	for _, row := range run.Rows {
		pdf.AddPage()
		writePayrollPayslipPage(pdf, *run, row, draftWatermark)
	}
	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, err
	}
	return bytes.Clone(buffer.Bytes()), nil
}

func writePayrollSummarySheet(f *excelize.File, run *model.PayrollRun) error {
	if err := setCells(f, "Summary", map[string]any{
		"A1": "Payroll Run",
		"A2": "Run ID",
		"B2": run.PayrollRunID,
		"A3": "Period",
		"B3": payrollRunPeriod(*run),
		"A4": "Status",
		"B4": string(run.Status),
	}); err != nil {
		return err
	}
	if err := writeRow(f, "Summary", 6, []any{"Staff", "Role", "Location", "Status", "Regular Minutes", "Overtime Minutes", "Gross", "Add", "Minus", "Final Pay"}); err != nil {
		return err
	}
	rowIndex := 7
	totals := payrollRunTotals(run.Rows)
	for _, row := range run.Rows {
		if err := writeRow(f, "Summary", rowIndex, []any{
			row.FullNameSnapshot,
			string(row.Role),
			payrollRowLocation(row),
			string(row.Status),
			row.RegularMinutes,
			row.OvertimeMinutes,
			int64(row.GrossCents),
			int64(row.AddAdjustmentsCents),
			int64(row.MinusAdjustmentsCents),
			int64(row.FinalPayCents),
		}); err != nil {
			return err
		}
		rowIndex++
	}
	return writeRow(f, "Summary", rowIndex+1, []any{"Totals", "", "", "", totals.regularMinutes, totals.overtimeMinutes, int64(totals.gross), int64(totals.add), int64(totals.minus), int64(totals.final)})
}

func writePayrollStaffRowsSheet(f *excelize.File, run *model.PayrollRun) error {
	headers := []any{"Run ID", "Row ID", "User ID", "Name", "Role", "Branch ID", "Location", "Status", "Regular Minutes", "Overtime Minutes", "Daily Rate", "Overtime Multiplier", "Gross", "Add", "Minus", "Final Pay"}
	if err := writeRow(f, "Staff Rows", 1, headers); err != nil {
		return err
	}
	for i, row := range run.Rows {
		branchID := ""
		if row.UsualBranchIDSnapshot != nil {
			branchID = fmt.Sprintf("%d", *row.UsualBranchIDSnapshot)
		}
		dailyRate := ""
		if row.DailyRateCents != nil {
			dailyRate = fmt.Sprintf("%d", *row.DailyRateCents)
		}
		multiplier := ""
		if row.OvertimeMultiplier != nil {
			multiplier = fmt.Sprintf("%.2f", *row.OvertimeMultiplier)
		}
		if err := writeRow(f, "Staff Rows", i+2, []any{
			run.PayrollRunID,
			row.PayrollRowID,
			row.UserID,
			row.FullNameSnapshot,
			string(row.Role),
			branchID,
			row.UsualLocationLabelSnapshot,
			string(row.Status),
			row.RegularMinutes,
			row.OvertimeMinutes,
			dailyRate,
			multiplier,
			int64(row.GrossCents),
			int64(row.AddAdjustmentsCents),
			int64(row.MinusAdjustmentsCents),
			int64(row.FinalPayCents),
		}); err != nil {
			return err
		}
	}
	return nil
}

func writePayrollBlockersSheet(f *excelize.File, run *model.PayrollRun) error {
	if err := writeRow(f, "Blockers", 1, []any{"Run ID", "Row ID", "User ID", "Name", "Role", "Blocker Code"}); err != nil {
		return err
	}
	rowIndex := 2
	for _, row := range run.Rows {
		for _, code := range row.BlockerCodes {
			if err := writeRow(f, "Blockers", rowIndex, []any{run.PayrollRunID, row.PayrollRowID, row.UserID, row.FullNameSnapshot, string(row.Role), code}); err != nil {
				return err
			}
			rowIndex++
		}
	}
	return nil
}

func writePayrollAttendanceSheet(f *excelize.File, run *model.PayrollRun) error {
	if err := writeRow(f, "Attendance Details", 1, []any{"Source", "Run ID", "Row ID", "User ID", "Name", "Role", "Date", "Attendance ID", "Time In", "Time Out", "Worked Minutes", "Regular Minutes", "Overtime Minutes", "Daily Rate", "Overtime Multiplier", "Gross"}); err != nil {
		return err
	}
	rowIndex := 2
	for _, row := range run.Rows {
		for _, detail := range row.AttendanceDetails {
			dailyRate := ""
			if detail.DailyRateCents != nil {
				dailyRate = fmt.Sprintf("%d", *detail.DailyRateCents)
			}
			multiplier := ""
			if detail.OvertimeMultiplier != nil {
				multiplier = fmt.Sprintf("%.2f", *detail.OvertimeMultiplier)
			}
			if err := writeRow(f, "Attendance Details", rowIndex, []any{
				"attendance",
				run.PayrollRunID,
				row.PayrollRowID,
				row.UserID,
				row.FullNameSnapshot,
				string(row.Role),
				payrollDetailDate(detail.Date, detail.WorkDate),
				detail.AttendanceID,
				formatPayrollTime(detail.TimeInAt),
				formatPayrollTime(detail.TimeOutAt),
				detail.WorkedMinutes,
				detail.RegularMinutes,
				detail.OvertimeMinutes,
				dailyRate,
				multiplier,
				int64(detail.GrossCents),
			}); err != nil {
				return err
			}
			rowIndex++
		}
	}
	return nil
}

func writePayrollBookingSheet(f *excelize.File, run *model.PayrollRun) error {
	if err := writeRow(f, "Booking Details", 1, []any{"Source", "Run ID", "Row ID", "User ID", "Name", "Role", "Date", "Booking ID", "Service", "Minutes", "Final Total", "Therapist Earnings"}); err != nil {
		return err
	}
	rowIndex := 2
	for _, row := range run.Rows {
		for _, detail := range row.BookingDetails {
			if err := writeRow(f, "Booking Details", rowIndex, []any{
				"booking",
				run.PayrollRunID,
				row.PayrollRowID,
				row.UserID,
				row.FullNameSnapshot,
				string(row.Role),
				payrollDetailDate(detail.Date, detail.BusinessDate),
				detail.BookingID,
				detail.ServiceName,
				detail.DurationMinutes,
				int64(detail.FinalTotalCents),
				int64(detail.TherapistEarningsCents),
			}); err != nil {
				return err
			}
			rowIndex++
		}
	}
	return nil
}

func writePayrollAdjustmentsSheet(f *excelize.File, run *model.PayrollRun) error {
	if err := writeRow(f, "Adjustments", 1, []any{"Run ID", "Row ID", "User ID", "Name", "Role", "Date", "Adjustment ID", "Type", "Category", "Amount", "Reason"}); err != nil {
		return err
	}
	rowIndex := 2
	for _, row := range run.Rows {
		for _, detail := range row.AdjustmentDetails {
			if err := writeRow(f, "Adjustments", rowIndex, []any{
				run.PayrollRunID,
				row.PayrollRowID,
				row.UserID,
				row.FullNameSnapshot,
				string(row.Role),
				payrollDetailDate(detail.Date, detail.AdjustmentDate),
				detail.AdjustmentID,
				string(detail.Type),
				detail.Category,
				int64(detail.AmountCents),
				detail.Reason,
			}); err != nil {
				return err
			}
			rowIndex++
		}
	}
	return nil
}

func writePayrollSettlementsSheet(f *excelize.File, run *model.PayrollRun) error {
	if err := writeRow(f, "Settlements", 1, []any{"Run ID", "Row ID", "User ID", "Name", "Role", "Status", "Paid At", "Paid By", "Method", "Reference", "Notes", "Ledger Entry ID", "Amount"}); err != nil {
		return err
	}
	rowIndex := 2
	for _, row := range run.Rows {
		paidBy := ""
		if row.PaidBy != nil {
			paidBy = fmt.Sprintf("%d", *row.PaidBy)
		}
		ledgerEntryID := ""
		if row.LedgerEntryID != nil {
			ledgerEntryID = fmt.Sprintf("%d", *row.LedgerEntryID)
		}
		if err := writeRow(f, "Settlements", rowIndex, []any{
			run.PayrollRunID,
			row.PayrollRowID,
			row.UserID,
			row.FullNameSnapshot,
			string(row.Role),
			string(row.Status),
			formatPayrollTime(row.PaidAt),
			paidBy,
			string(row.PaymentMethod),
			row.PaymentReference,
			row.PaymentNotes,
			ledgerEntryID,
			int64(row.FinalPayCents),
		}); err != nil {
			return err
		}
		rowIndex++
	}
	return nil
}

type payrollTotals struct {
	regularMinutes  int
	overtimeMinutes int
	gross           model.PayrollMoneyCents
	add             model.PayrollMoneyCents
	minus           model.PayrollMoneyCents
	final           model.PayrollMoneyCents
}

func payrollRunTotals(rows []model.PayrollRow) payrollTotals {
	var totals payrollTotals
	for _, row := range rows {
		totals.regularMinutes += row.RegularMinutes
		totals.overtimeMinutes += row.OvertimeMinutes
		totals.gross += row.GrossCents
		totals.add += row.AddAdjustmentsCents
		totals.minus += row.MinusAdjustmentsCents
		totals.final += row.FinalPayCents
	}
	return totals
}

func writePayrollPayslipPage(pdf *gofpdf.Fpdf, run model.PayrollRun, row model.PayrollRow, draftWatermark bool) {
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 8, "Relaxation Hub")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, "Payroll Period: "+payrollRunPeriod(run))
	pdf.Ln(6)
	pdf.Cell(0, 6, "Run Status: "+string(run.Status))
	pdf.Ln(7)
	if draftWatermark || run.Status == model.PayrollRunStatusDraft {
		pdf.SetFont("Helvetica", "B", 18)
		pdf.Cell(0, 9, "DRAFT")
		pdf.Ln(10)
	}

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 7, "Staff")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, "Name: "+row.FullNameSnapshot)
	pdf.Ln(6)
	pdf.Cell(0, 6, "Role: "+payrollRoleLabel(row.Role))
	pdf.Ln(6)
	pdf.Cell(0, 6, "Branch/location: "+payrollRowLocation(row))
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 7, "Source Rows")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 9)
	if row.Role == model.PayrollRoleRider || row.Role == model.PayrollRoleAdmin {
		for _, detail := range row.AttendanceDetails {
			pdf.Cell(0, 5, fmt.Sprintf("attendance %s regular minutes %d overtime minutes %d gross %s", payrollDetailDate(detail.Date, detail.WorkDate), detail.RegularMinutes, detail.OvertimeMinutes, payrollMoney(detail.GrossCents)))
			pdf.Ln(5)
		}
	}
	if row.Role == model.PayrollRoleTherapist {
		for _, detail := range row.BookingDetails {
			pdf.Cell(0, 5, fmt.Sprintf("booking %s %s minutes %d earnings %s", payrollDetailDate(detail.Date, detail.BusinessDate), detail.ServiceName, detail.DurationMinutes, payrollMoney(detail.TherapistEarningsCents)))
			pdf.Ln(5)
		}
	}
	for _, detail := range row.AdjustmentDetails {
		pdf.Cell(0, 5, fmt.Sprintf("adjustment %s %s %s %s reason %s", payrollDetailDate(detail.Date, detail.AdjustmentDate), string(detail.Type), detail.Category, payrollMoney(detail.AmountCents), detail.Reason))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 7, "Totals")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("regular minutes: %d", row.RegularMinutes))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("overtime minutes: %d", row.OvertimeMinutes))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("gross: %s add: %s minus: %s final pay: %s", payrollMoney(row.GrossCents), payrollMoney(row.AddAdjustmentsCents), payrollMoney(row.MinusAdjustmentsCents), payrollMoney(row.FinalPayCents)))
	pdf.Ln(7)
	if row.DailyRateCents != nil || row.OvertimeMultiplier != nil {
		pdf.Cell(0, 6, "Rate summary: "+payrollRateSummary(row))
		pdf.Ln(7)
	}

	pdf.Ln(12)
	pdf.Cell(0, 6, "Acknowledgment")
	pdf.Ln(12)
	pdf.Cell(85, 6, "Staff signature: ____________________")
	pdf.Cell(85, 6, "Date: ____________________")
}

func payrollRunPeriod(run model.PayrollRun) string {
	start := run.StartDate
	if start == "" && !run.PeriodStart.IsZero() {
		start = run.PeriodStart.Format("2006-01-02")
	}
	end := run.EndDate
	if end == "" && !run.PeriodEnd.IsZero() {
		end = run.PeriodEnd.Format("2006-01-02")
	}
	return start + " to " + end
}

func payrollDetailDate(snapshot string, value time.Time) string {
	if strings.TrimSpace(snapshot) != "" {
		return snapshot
	}
	if !value.IsZero() {
		return value.Format("2006-01-02")
	}
	return ""
}

func payrollRowLocation(row model.PayrollRow) string {
	location := strings.TrimSpace(row.UsualLocationLabelSnapshot)
	if location != "" {
		return location
	}
	if row.UsualBranchIDSnapshot != nil {
		return fmt.Sprintf("Branch %d", *row.UsualBranchIDSnapshot)
	}
	return "Unassigned"
}

func payrollRoleLabel(role model.PayrollRole) string {
	switch role {
	case model.PayrollRoleRider:
		return "Rider"
	case model.PayrollRoleAdmin:
		return "Admin"
	case model.PayrollRoleTherapist:
		return "Therapist"
	default:
		return string(role)
	}
}

func payrollMoney(value model.PayrollMoneyCents) string {
	return fmt.Sprintf("%.2f", float64(value)/100)
}

func payrollRateSummary(row model.PayrollRow) string {
	parts := make([]string, 0, 2)
	if row.DailyRateCents != nil {
		parts = append(parts, "daily rate "+payrollMoney(*row.DailyRateCents))
	}
	if row.OvertimeMultiplier != nil {
		parts = append(parts, fmt.Sprintf("overtime multiplier %.2f", *row.OvertimeMultiplier))
	}
	return strings.Join(parts, ", ")
}

func formatPayrollTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

type payrollDraftRow struct {
	model.PayrollRow
	blockers          map[string]struct{}
	attendanceDetails []model.PayrollAttendanceDetail
	bookingDetails    []model.PayrollBookingDetail
	adjustmentDetails []model.PayrollAdjustmentDetail
}

func payrollDraftRowFor(rows map[int64]*payrollDraftRow, runID, userID int64, role model.PayrollRole, fullName string, branchID *int64, locationLabel string) *payrollDraftRow {
	if existing := rows[userID]; existing != nil {
		if existing.FullNameSnapshot == "" {
			existing.FullNameSnapshot = fullName
		}
		if existing.UsualBranchIDSnapshot == nil {
			existing.UsualBranchIDSnapshot = branchID
		}
		if existing.UsualLocationLabelSnapshot == "" {
			existing.UsualLocationLabelSnapshot = strings.TrimSpace(locationLabel)
		}
		return existing
	}
	row := &payrollDraftRow{
		PayrollRow: model.PayrollRow{
			PayrollRunID:               runID,
			UserID:                     userID,
			Role:                       role,
			FullNameSnapshot:           fullName,
			UsualBranchIDSnapshot:      branchID,
			UsualLocationLabelSnapshot: strings.TrimSpace(locationLabel),
		},
		blockers: make(map[string]struct{}),
	}
	rows[userID] = row
	return row
}

func (r *payrollDraftRow) addBlocker(code string) {
	if strings.TrimSpace(code) == "" {
		return
	}
	r.blockers[code] = struct{}{}
}

func (r *payrollDraftRow) blockerCodes() []string {
	codes := make([]string, 0, len(r.blockers))
	for code := range r.blockers {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func prepareStaffPayrollAdjustment(adjustment model.StaffPayrollAdjustment) (model.StaffPayrollAdjustment, error) {
	if !isPayrollAdjustmentRole(adjustment.Role) {
		return model.StaffPayrollAdjustment{}, model.ErrInvalidPayrollRole
	}
	if adjustment.UserID <= 0 ||
		adjustment.AdjustmentDate.IsZero() ||
		adjustment.PeriodStart.IsZero() ||
		adjustment.PeriodEnd.IsZero() ||
		adjustment.PeriodEnd.Before(adjustment.PeriodStart) ||
		adjustment.AmountCents <= 0 ||
		strings.TrimSpace(adjustment.Reason) == "" {
		return model.StaffPayrollAdjustment{}, model.ErrInvalidPayrollAdjustment
	}
	if adjustment.Type != model.PayrollAdjustmentTypeAdd && adjustment.Type != model.PayrollAdjustmentTypeMinus {
		return model.StaffPayrollAdjustment{}, model.ErrInvalidPayrollAdjustment
	}
	adjustment.Category = strings.TrimSpace(adjustment.Category)
	if !isPayrollAdjustmentCategory(adjustment.Category) {
		return model.StaffPayrollAdjustment{}, model.ErrInvalidPayrollAdjustment
	}
	adjustment.Reason = strings.TrimSpace(adjustment.Reason)
	return adjustment, nil
}

func isPayrollAdjustmentCategory(category string) bool {
	switch category {
	case "benefits", "cash_advance", "salary_correction", "attendance_correction", "bonus", "deduction", "parcel", "absence", "other":
		return true
	default:
		return false
	}
}

func isPayableRateRole(role model.PayrollRole) bool {
	switch string(role) {
	case model.RoleRider, model.RoleAdmin:
		return true
	default:
		return false
	}
}

func isPayrollAdjustmentRole(role model.PayrollRole) bool {
	switch string(role) {
	case model.RoleTherapist, model.RoleRider, model.RoleAdmin:
		return true
	default:
		return false
	}
}

func isValidPayrollPaymentMethod(method string) bool {
	switch model.PayrollPaymentMethod(method) {
	case model.PayrollPaymentMethodCash,
		model.PayrollPaymentMethodGCash,
		model.PayrollPaymentMethodBankTransfer,
		model.PayrollPaymentMethodOther:
		return true
	default:
		return false
	}
}
