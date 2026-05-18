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
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type PayrollHandler struct {
	payrollService service.PayrollService
}

func NewPayrollHandler(payrollService service.PayrollService) *PayrollHandler {
	return &PayrollHandler{payrollService: payrollService}
}

func (h *PayrollHandler) ListCompensationRates(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseOptionalPayrollUserID(w, r)
	if !ok {
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	rates, err := h.payrollService.ListCompensationRates(r.Context(), userID, role)
	if err != nil {
		writePayrollError(w, err, "Failed to list rates")
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.StaffCompensationRate `json:"data"`
	}{Data: rates})
}

func (h *PayrollHandler) CreateCompensationRate(w http.ResponseWriter, r *http.Request) {
	rate, ok := decodeCompensationRateRequest(w, r)
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	created, err := h.payrollService.CreateCompensationRate(r.Context(), rate, actorID)
	if err != nil {
		writePayrollError(w, err, "Failed to create rate")
		return
	}
	writeReportJSON(w, http.StatusCreated, created)
}

func (h *PayrollHandler) ListStaffPayrollAdjustments(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseStaffPayrollAdjustmentFilter(w, r)
	if !ok {
		return
	}
	items, err := h.payrollService.ListStaffPayrollAdjustments(r.Context(), filter)
	if err != nil {
		writePayrollError(w, err, "Failed to list adjustments")
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.StaffPayrollAdjustment `json:"data"`
	}{Data: items})
}

func (h *PayrollHandler) CreateStaffPayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	adjustment, ok := decodeStaffPayrollAdjustmentRequest(w, r, 0)
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	created, err := h.payrollService.CreateStaffPayrollAdjustment(r.Context(), adjustment, actorID)
	if err != nil {
		writePayrollError(w, err, "Failed to create adjustment")
		return
	}
	writeReportJSON(w, http.StatusCreated, created)
}

func (h *PayrollHandler) UpdateStaffPayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	adjustment, ok := decodeStaffPayrollAdjustmentRequest(w, r, id)
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	updated, err := h.payrollService.UpdateStaffPayrollAdjustment(r.Context(), adjustment, actorID)
	if err != nil {
		writePayrollError(w, err, "Failed to update adjustment")
		return
	}
	writeReportJSON(w, http.StatusOK, updated)
}

func (h *PayrollHandler) DeleteStaffPayrollAdjustment(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.payrollService.VoidStaffPayrollAdjustment(r.Context(), id, actorID); err != nil {
		writePayrollError(w, err, "Failed to void adjustment")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) UpsertStaffProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := parsePathID(w, r, "userID")
	if !ok {
		return
	}
	var req struct {
		UsualBranchID      *int64 `json:"usual_branch_id"`
		UsualLocationLabel string `json:"usual_location_label"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UsualBranchID != nil && *req.UsualBranchID <= 0 {
		http.Error(w, "Invalid usual_branch_id", http.StatusBadRequest)
		return
	}
	if err := h.payrollService.UpsertStaffProfile(r.Context(), userID, req.UsualBranchID, strings.TrimSpace(req.UsualLocationLabel)); err != nil {
		writePayrollError(w, err, "Failed to update staff profile")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) CreatePayrollRun(w http.ResponseWriter, r *http.Request) {
	filter, ok := decodePayrollGenerationRequest(w, r)
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	filter.GeneratedBy = actorID
	run, err := h.payrollService.GenerateDraftPayrollRun(r.Context(), filter)
	if err != nil {
		writePayrollError(w, err, "Failed to generate payroll run")
		return
	}
	writeReportJSON(w, http.StatusCreated, run)
}

func (h *PayrollHandler) ListPayrollRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.payrollService.ListPayrollRuns(r.Context())
	if err != nil {
		writePayrollError(w, err, "Failed to list payroll runs")
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.PayrollRun `json:"data"`
	}{Data: runs})
}

func (h *PayrollHandler) GetPayrollRun(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	run, err := h.payrollService.GetPayrollRun(r.Context(), id)
	if err != nil {
		writePayrollError(w, err, "Failed to get payroll run")
		return
	}
	writeReportJSON(w, http.StatusOK, run)
}

func (h *PayrollHandler) ApprovePayrollRun(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.payrollService.ApprovePayrollRun(r.Context(), id, actorID); err != nil {
		writePayrollError(w, err, "Failed to approve payroll run")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) VoidPayrollRun(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.Body != http.NoBody {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}
	if err := h.payrollService.VoidPayrollRun(r.Context(), id, actorID, strings.TrimSpace(req.Reason)); err != nil {
		writePayrollError(w, err, "Failed to void payroll run")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) MarkPayrollRowPaid(w http.ResponseWriter, r *http.Request) {
	runID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	rowID, ok := parsePathID(w, r, "rowID")
	if !ok {
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	req, ok := decodePayrollPaymentRequest(w, r)
	if !ok {
		return
	}
	row, err := h.payrollService.MarkPayrollRowPaid(r.Context(), runID, rowID, actorID, req)
	if err != nil {
		writePayrollError(w, err, "Failed to mark payroll row paid")
		return
	}
	writeReportJSON(w, http.StatusOK, row)
}

func (h *PayrollHandler) CheckPayrollRunStaleness(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	stale, reasons, err := h.payrollService.CheckPayrollRunStaleness(r.Context(), id)
	if err != nil {
		writePayrollError(w, err, "Failed to check payroll run staleness")
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Stale   bool     `json:"stale"`
		Reasons []string `json:"reasons"`
	}{Stale: stale, Reasons: reasons})
}

func (h *PayrollHandler) ExportPayrollWorkbook(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	workbook, err := h.payrollService.BuildPayrollWorkbook(r.Context(), id)
	if err != nil {
		writePayrollError(w, err, "Failed to export payroll workbook")
		return
	}
	writePayrollBinary(w, workbook, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", fmt.Sprintf("payroll-run-%d.xlsx", id))
}

func (h *PayrollHandler) ExportPayrollPayslipPDF(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	pdf, err := h.payrollService.BuildPayrollPayslipPDF(r.Context(), id, false)
	if err != nil {
		writePayrollError(w, err, "Failed to export payroll payslips")
		return
	}
	writePayrollBinary(w, pdf, "application/pdf", fmt.Sprintf("payroll-run-%d-payslips.pdf", id))
}

func decodePayrollPaymentRequest(w http.ResponseWriter, r *http.Request) (model.PayrollPaymentRequest, bool) {
	var req model.PayrollPaymentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.PayrollPaymentRequest{}, false
	}
	req.PaymentMethod = model.PayrollPaymentMethod(strings.TrimSpace(string(req.PaymentMethod)))
	req.PaymentReference = strings.TrimSpace(req.PaymentReference)
	req.PaymentNotes = strings.TrimSpace(req.PaymentNotes)
	return req, true
}

func decodePayrollGenerationRequest(w http.ResponseWriter, r *http.Request) (model.PayrollGenerationFilter, bool) {
	var req struct {
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.PayrollGenerationFilter{}, false
	}
	periodStart, err := time.Parse("2006-01-02", strings.TrimSpace(req.PeriodStart))
	if err != nil {
		http.Error(w, "Invalid period_start format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.PayrollGenerationFilter{}, false
	}
	periodEnd, err := time.Parse("2006-01-02", strings.TrimSpace(req.PeriodEnd))
	if err != nil || periodEnd.Before(periodStart) {
		http.Error(w, "Invalid period_end format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.PayrollGenerationFilter{}, false
	}
	return model.PayrollGenerationFilter{PeriodStart: periodStart, PeriodEnd: periodEnd}, true
}

func decodeCompensationRateRequest(w http.ResponseWriter, r *http.Request) (model.StaffCompensationRate, bool) {
	var req struct {
		UserID             int64   `json:"user_id"`
		Role               string  `json:"role"`
		DailyRateCents     int64   `json:"daily_rate_cents"`
		OvertimeMultiplier float64 `json:"overtime_multiplier"`
		EffectiveFrom      string  `json:"effective_from"`
		EffectiveTo        *string `json:"effective_to"`
		Notes              string  `json:"notes"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.StaffCompensationRate{}, false
	}
	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		http.Error(w, "Invalid effective_from format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.StaffCompensationRate{}, false
	}
	var effectiveTo *time.Time
	if req.EffectiveTo != nil && strings.TrimSpace(*req.EffectiveTo) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.EffectiveTo))
		if err != nil || parsed.Before(effectiveFrom) {
			http.Error(w, "Invalid effective_to format (YYYY-MM-DD)", http.StatusBadRequest)
			return model.StaffCompensationRate{}, false
		}
		effectiveTo = &parsed
	}
	if req.UserID <= 0 || req.DailyRateCents <= 0 {
		http.Error(w, "Invalid compensation rate payload", http.StatusBadRequest)
		return model.StaffCompensationRate{}, false
	}
	return model.StaffCompensationRate{
		UserID:             req.UserID,
		Role:               model.PayrollRole(strings.TrimSpace(req.Role)),
		DailyRateCents:     model.PayrollMoneyCents(req.DailyRateCents),
		OvertimeMultiplier: req.OvertimeMultiplier,
		EffectiveFrom:      effectiveFrom,
		EffectiveFromDate:  effectiveFrom.Format("2006-01-02"),
		EffectiveTo:        effectiveTo,
		Notes:              strings.TrimSpace(req.Notes),
	}, true
}

func decodeStaffPayrollAdjustmentRequest(w http.ResponseWriter, r *http.Request, adjustmentID int64) (model.StaffPayrollAdjustment, bool) {
	var req struct {
		UserID            int64  `json:"user_id"`
		Role              string `json:"role"`
		AdjustmentDate    string `json:"adjustment_date"`
		PeriodStart       string `json:"period_start"`
		PeriodEnd         string `json:"period_end"`
		Type              string `json:"type"`
		Category          string `json:"category"`
		AmountCents       int64  `json:"amount_cents"`
		Reason            string `json:"reason"`
		CashMovementCents int64  `json:"cash_movement_cents"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.StaffPayrollAdjustment{}, false
	}
	adjustmentDate, err := time.Parse("2006-01-02", req.AdjustmentDate)
	if err != nil {
		http.Error(w, "Invalid adjustment_date format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.StaffPayrollAdjustment{}, false
	}
	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		http.Error(w, "Invalid period_start format (YYYY-MM-DD)", http.StatusBadRequest)
		return model.StaffPayrollAdjustment{}, false
	}
	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil || periodEnd.Before(periodStart) || req.UserID <= 0 || req.AmountCents <= 0 || strings.TrimSpace(req.Reason) == "" {
		http.Error(w, "Invalid payroll adjustment payload", http.StatusBadRequest)
		return model.StaffPayrollAdjustment{}, false
	}
	return model.StaffPayrollAdjustment{
		AdjustmentID:      adjustmentID,
		UserID:            req.UserID,
		Role:              model.PayrollRole(strings.TrimSpace(req.Role)),
		AdjustmentDate:    adjustmentDate,
		Date:              adjustmentDate.Format("2006-01-02"),
		PeriodStart:       periodStart,
		PeriodStartDate:   periodStart.Format("2006-01-02"),
		PeriodEnd:         periodEnd,
		PeriodEndDate:     periodEnd.Format("2006-01-02"),
		Type:              model.PayrollAdjustmentType(strings.TrimSpace(req.Type)),
		Category:          strings.TrimSpace(req.Category),
		AmountCents:       model.PayrollMoneyCents(req.AmountCents),
		Reason:            strings.TrimSpace(req.Reason),
		CashMovementCents: model.PayrollMoneyCents(req.CashMovementCents),
	}, true
}

func parseStaffPayrollAdjustmentFilter(w http.ResponseWriter, r *http.Request) (repository.StaffPayrollAdjustmentFilter, bool) {
	periodStart, ok := parseRequiredReportDate(w, r, "period_start")
	if !ok {
		return repository.StaffPayrollAdjustmentFilter{}, false
	}
	periodEnd, ok := parseRequiredReportDate(w, r, "period_end")
	if !ok {
		return repository.StaffPayrollAdjustmentFilter{}, false
	}
	if periodEnd.Before(periodStart) {
		http.Error(w, "period_end must be on or after period_start", http.StatusBadRequest)
		return repository.StaffPayrollAdjustmentFilter{}, false
	}
	userID, ok := parseOptionalPayrollUserID(w, r)
	if !ok {
		return repository.StaffPayrollAdjustmentFilter{}, false
	}
	return repository.StaffPayrollAdjustmentFilter{
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		UserID:      userID,
		Role:        strings.TrimSpace(r.URL.Query().Get("role")),
	}, true
}

func parseOptionalPayrollUserID(w http.ResponseWriter, r *http.Request) (*int64, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		http.Error(w, "Invalid user_id", http.StatusBadRequest)
		return nil, false
	}
	return &parsed, true
}

func writePayrollError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		http.Error(w, "Payroll record not found", http.StatusNotFound)
	case errors.Is(err, model.ErrInvalidPayrollRole),
		errors.Is(err, model.ErrInvalidPayrollRate),
		errors.Is(err, model.ErrInvalidPayrollAdjustment),
		errors.Is(err, model.ErrPayrollPaymentMethodRequired),
		errors.Is(err, model.ErrInvalidPayrollPaymentMethod):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, model.ErrPayrollRateLocked),
		errors.Is(err, model.ErrPayrollAdjustmentLocked),
		errors.Is(err, model.ErrPayrollRunHasBlockers),
		errors.Is(err, model.ErrPayrollRunImmutable):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writePayrollBinary(w http.ResponseWriter, body []byte, contentType string, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
