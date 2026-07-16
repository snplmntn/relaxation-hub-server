package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

const maxReportRangeDays = 62

func (h *ReportHandler) GetBookingExportReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationGetBookingExportReport) {
		return
	}
	filter, ok := parseBookingExportFilter(w, r)
	if !ok {
		return
	}
	report, err := h.reportExportService.BuildBookingExportReport(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to build booking export report", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) ExportBookingReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationExportBookingReport) {
		return
	}
	filter, ok := parseBookingExportFilter(w, r)
	if !ok {
		return
	}
	report, err := h.reportExportService.BuildBookingExportReport(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to build booking export report", http.StatusInternalServerError)
		return
	}
	workbook, err := h.reportExportService.BuildBookingExportWorkbook(*report)
	if err != nil {
		http.Error(w, "Failed to export booking report", http.StatusInternalServerError)
		return
	}
	writeWorkbook(w, fmt.Sprintf("therapist-payroll-%s-%s.xlsx", filter.StartDate.Format("2006-01-02"), filter.EndDate.Format("2006-01-02")), workbook)
}

func (h *ReportHandler) GetDailySalesReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationGetDailySalesReport) {
		return
	}
	businessDate, ok := parseRequiredReportDate(w, r, "business_date")
	if !ok {
		return
	}

	report, err := h.reportExportService.BuildDailySalesReport(r.Context(), businessDate)
	if err != nil {
		http.Error(w, "Failed to build daily sales report", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) UpsertDailySalesRemittance(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationUpsertDailySalesRemittance) {
		return
	}
	var req model.UpsertDailySalesRemittanceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	businessDate, err := time.Parse("2006-01-02", req.BusinessDate)
	if err != nil || req.BranchID <= 0 || !validRemittancePayload(req) {
		http.Error(w, "business_date and branch_id are required", http.StatusBadRequest)
		return
	}
	actorID, _ := middleware.GetUserID(r)
	remittance := model.DailySalesRemittance{
		BusinessDate:        businessDate,
		Date:                req.BusinessDate,
		BranchID:            req.BranchID,
		Bill1000:            req.Bill1000,
		Bill500:             req.Bill500,
		Bill200:             req.Bill200,
		Bill100:             req.Bill100,
		Bill50:              req.Bill50,
		Bill20:              req.Bill20,
		Bill10:              req.Bill10,
		Bill5:               req.Bill5,
		Bill1:               req.Bill1,
		ActualRemitted:      req.ActualRemitted,
		TipsTotal:           req.TipsTotal,
		ClientFundsUsed:     req.ClientFundsUsed,
		ClientFundsAdded:    req.ClientFundsAdded,
		RemittedToMark:      req.RemittedToMark,
		OtherRemittedAmount: req.OtherRemittedAmount,
		RemittedTo:          strings.TrimSpace(req.RemittedTo),
		OthersDeducted:      req.OthersDeducted,
		OthersAdded:         req.OthersAdded,
		Notes:               strings.TrimSpace(req.Notes),
		CreatedBy:           &actorID,
		UpdatedBy:           &actorID,
	}
	stored, err := h.reportExportService.UpsertDailySalesRemittance(r.Context(), remittance)
	if err != nil {
		http.Error(w, "Failed to save remittance", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, stored)
}

func (h *ReportHandler) ExportDailySalesReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationExportDailySalesReport) {
		return
	}
	businessDate, ok := parseRequiredReportDate(w, r, "business_date")
	if !ok {
		return
	}
	report, err := h.reportExportService.BuildDailySalesReport(r.Context(), businessDate)
	if err != nil {
		http.Error(w, "Failed to build daily sales report", http.StatusInternalServerError)
		return
	}
	workbook, err := h.reportExportService.BuildDailySalesWorkbook(*report)
	if err != nil {
		http.Error(w, "Failed to export daily sales report", http.StatusInternalServerError)
		return
	}
	writeWorkbook(w, fmt.Sprintf("daily-sales-%s.xlsx", businessDate.Format("2006-01-02")), workbook)
}

func (h *ReportHandler) ListPayrollAdjustments(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationListPayrollAdjustments) {
		return
	}
	filter, ok := parsePayrollAdjustmentFilter(w, r)
	if !ok {
		return
	}
	items, err := h.reportExportService.ListPayrollAdjustments(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to list payroll adjustments", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.PayrollAdjustment `json:"data"`
	}{Data: items})
}

func (h *ReportHandler) CreatePayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationCreatePayrollAdjustment) {
		return
	}
	adjustment, ok := decodePayrollAdjustmentRequest(w, r, 0)
	if !ok {
		return
	}
	created, err := h.reportExportService.CreatePayrollAdjustment(r.Context(), adjustment)
	if err != nil {
		http.Error(w, "Failed to create payroll adjustment", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusCreated, created)
}

func (h *ReportHandler) UpdatePayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationUpdatePayrollAdjustment) {
		return
	}
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	adjustment, ok := decodePayrollAdjustmentRequest(w, r, id)
	if !ok {
		return
	}
	updated, err := h.reportExportService.UpdatePayrollAdjustment(r.Context(), adjustment)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "Payroll adjustment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to update payroll adjustment", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, updated)
}

func (h *ReportHandler) DeletePayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationDeletePayrollAdjustment) {
		return
	}
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, _ := middleware.GetUserID(r)
	if err := h.reportExportService.VoidPayrollAdjustment(r.Context(), id, actorID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "Payroll adjustment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to void payroll adjustment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ReportHandler) ExportTherapistSalaries(w http.ResponseWriter, r *http.Request) {
	if !h.requireReportDependencies(w, r, reportOperationExportTherapistSalaries) {
		return
	}
	filter, ok := parseSalaryReportFilter(w, r)
	if !ok {
		return
	}
	report, err := h.reportExportService.BuildSalaryReport(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to build therapist salary report", http.StatusInternalServerError)
		return
	}
	workbook, err := h.reportExportService.BuildSalaryWorkbook(*report)
	if err != nil {
		http.Error(w, "Failed to export therapist salary report", http.StatusInternalServerError)
		return
	}
	writeWorkbook(w, fmt.Sprintf("therapist-salaries-%s-%s.xlsx", filter.StartDate.Format("2006-01-02"), filter.EndDate.Format("2006-01-02")), workbook)
}

func parseRequiredReportDate(w http.ResponseWriter, r *http.Request, name string) (time.Time, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		http.Error(w, name+" is required", http.StatusBadRequest)
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		http.Error(w, "Invalid "+name+" format (YYYY-MM-DD)", http.StatusBadRequest)
		return time.Time{}, false
	}
	return parsed, true
}

func parsePayrollAdjustmentFilter(w http.ResponseWriter, r *http.Request) (model.PayrollAdjustmentFilter, bool) {
	startDate, ok := parseRequiredReportDate(w, r, "start_date")
	if !ok {
		return model.PayrollAdjustmentFilter{}, false
	}
	endDate, ok := parseRequiredReportDate(w, r, "end_date")
	if !ok {
		return model.PayrollAdjustmentFilter{}, false
	}
	therapistID, ok := parseOptionalTherapistID(w, r)
	if !ok {
		return model.PayrollAdjustmentFilter{}, false
	}
	if !validateReportDateRange(w, startDate, endDate) {
		return model.PayrollAdjustmentFilter{}, false
	}
	return model.PayrollAdjustmentFilter{StartDate: startDate, EndDate: endDate, TherapistID: therapistID}, true
}

func parseSalaryReportFilter(w http.ResponseWriter, r *http.Request) (model.SalaryReportFilter, bool) {
	filter, ok := parsePayrollAdjustmentFilter(w, r)
	if !ok {
		return model.SalaryReportFilter{}, false
	}
	return model.SalaryReportFilter{StartDate: filter.StartDate, EndDate: filter.EndDate, TherapistID: filter.TherapistID}, true
}

func parseBookingExportFilter(w http.ResponseWriter, r *http.Request) (model.BookingExportFilter, bool) {
	filter, ok := parsePayrollAdjustmentFilter(w, r)
	if !ok {
		return model.BookingExportFilter{}, false
	}
	return model.BookingExportFilter{StartDate: filter.StartDate, EndDate: filter.EndDate, TherapistID: filter.TherapistID}, true
}

func parseOptionalTherapistID(w http.ResponseWriter, r *http.Request) (*int64, bool) {
	value := r.URL.Query().Get("therapist_id")
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		http.Error(w, "Invalid therapist_id", http.StatusBadRequest)
		return nil, false
	}
	return &parsed, true
}

func parsePathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	value := r.PathValue(name)
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		http.Error(w, "Invalid "+name, http.StatusBadRequest)
		return 0, false
	}
	return parsed, true
}

func decodePayrollAdjustmentRequest(w http.ResponseWriter, r *http.Request, adjustmentID int64) (model.PayrollAdjustment, bool) {
	var req model.PayrollAdjustmentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	adjustmentDate, err := time.Parse("2006-01-02", req.AdjustmentDate)
	if err != nil {
		http.Error(w, "Invalid adjustment_date format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		http.Error(w, "Invalid period_start format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil || periodEnd.Before(periodStart) || req.TherapistID <= 0 || req.Amount < 0 || strings.TrimSpace(req.Reason) == "" {
		http.Error(w, "Invalid payroll adjustment payload", http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	adjustmentType := model.PayrollAdjustmentType(req.Type)
	if adjustmentType != model.PayrollAdjustmentTypeAdd && adjustmentType != model.PayrollAdjustmentTypeMinus {
		http.Error(w, `type must be "add" or "minus"`, http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	category := model.PayrollAdjustmentCategory(req.Category)
	if !validPayrollAdjustmentCategory(category) {
		http.Error(w, "Invalid payroll adjustment category", http.StatusBadRequest)
		return model.PayrollAdjustment{}, false
	}
	actorID, _ := middleware.GetUserID(r)
	return model.PayrollAdjustment{
		AdjustmentID:    adjustmentID,
		TherapistID:     req.TherapistID,
		AdjustmentDate:  adjustmentDate,
		Date:            req.AdjustmentDate,
		PeriodStart:     periodStart,
		PeriodStartDate: req.PeriodStart,
		PeriodEnd:       periodEnd,
		PeriodEndDate:   req.PeriodEnd,
		Type:            adjustmentType,
		Category:        category,
		Amount:          req.Amount,
		Reason:          strings.TrimSpace(req.Reason),
		CashMovement:    req.CashMovement,
		CreatedBy:       &actorID,
		UpdatedBy:       &actorID,
	}, true
}

func validateReportDateRange(w http.ResponseWriter, startDate time.Time, endDate time.Time) bool {
	if endDate.Before(startDate) {
		http.Error(w, "end_date must be on or after start_date", http.StatusBadRequest)
		return false
	}
	if int(endDate.Sub(startDate).Hours()/24)+1 > maxReportRangeDays {
		http.Error(w, "date range exceeds maximum report range", http.StatusBadRequest)
		return false
	}
	return true
}

func validRemittancePayload(req model.UpsertDailySalesRemittanceRequest) bool {
	counts := []int{req.Bill1000, req.Bill500, req.Bill200, req.Bill100, req.Bill50, req.Bill20, req.Bill10, req.Bill5, req.Bill1}
	for _, count := range counts {
		if count < 0 {
			return false
		}
	}
	amounts := []float64{req.ActualRemitted, req.TipsTotal, req.ClientFundsUsed, req.ClientFundsAdded, req.RemittedToMark, req.OtherRemittedAmount, req.OthersDeducted, req.OthersAdded}
	for _, amount := range amounts {
		if amount < 0 {
			return false
		}
	}
	return true
}

func validPayrollAdjustmentCategory(category model.PayrollAdjustmentCategory) bool {
	switch category {
	case model.PayrollAdjustmentCategoryBenefits, model.PayrollAdjustmentCategoryCashAdvance, model.PayrollAdjustmentCategorySalary, model.PayrollAdjustmentCategoryCorrection, model.PayrollAdjustmentCategoryParcel, model.PayrollAdjustmentCategoryAbsence, model.PayrollAdjustmentCategoryOther:
		return true
	default:
		return false
	}
}

func writeReportJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeWorkbook(w http.ResponseWriter, filename string, workbook []byte) {
	w.Header().Set("Content-Type", model.ExcelContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(workbook)
}
