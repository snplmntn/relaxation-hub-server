package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// ReportHandler handles accounting and reporting endpoints.
type ReportHandler struct {
	bookingRepo       repository.BookingRepository
	ledgerRepo        repository.LedgerRepository
	storageService    service.StorageService
	riderWalletService *service.RiderWalletService
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(br repository.BookingRepository, lr repository.LedgerRepository, ss service.StorageService, rws *service.RiderWalletService) *ReportHandler {
	return &ReportHandler{bookingRepo: br, ledgerRepo: lr, storageService: ss, riderWalletService: rws}
}

// AccountingSummaryResponse is the response for accounting summary.
type AccountingSummaryResponse struct {
	TotalRevenue          float64 `json:"total_revenue"`
	TotalTherapistPayouts float64 `json:"total_therapist_payouts"`
	TotalPlatformProfit   float64 `json:"total_platform_profit"`
	BookingCount          int     `json:"booking_count"`
	StartDate             string  `json:"start_date"`
	EndDate               string  `json:"end_date"`
}

// LedgerSummaryResponse is the response for ledger-based summary.
type LedgerSummaryResponse struct {
	TotalCredits float64 `json:"total_credits"`
	TotalDebits  float64 `json:"total_debits"`
	NetProfit    float64 `json:"net_profit"`
	EntryCount   int     `json:"entry_count"`
	StartDate    string  `json:"start_date"`
	EndDate      string  `json:"end_date"`
}

// LedgerPeriodEntry represents a single period's ledger data.
type LedgerPeriodEntry struct {
	PeriodStart  string  `json:"period_start"`
	TotalCredits float64 `json:"total_credits"`
	TotalDebits  float64 `json:"total_debits"`
	NetProfit    float64 `json:"net_profit"`
	EntryCount   int     `json:"entry_count"`
}

// DailyAccountingEntry represents a single day's accounting data (legacy).
type DailyAccountingEntry struct {
	Date             string  `json:"date"`
	Revenue          float64 `json:"revenue"`
	TherapistPayouts float64 `json:"therapist_payouts"`
	PlatformProfit   float64 `json:"platform_profit"`
	BookingCount     int     `json:"booking_count"`
}

// parseDateRange extracts start_date and end_date from query params, defaulting to last 30 days.
func parseDateRange(r *http.Request) (time.Time, time.Time) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			// Set to end of day
			endDate = parsed.Add(24*time.Hour - time.Second)
		}
	}
	return startDate, endDate
}

// GetAccountingSummary returns aggregated accounting data for a date range (legacy, from bookings).
// GET /admin/reports/accounting/summary?start_date=...&end_date=...
func (h *ReportHandler) GetAccountingSummary(w http.ResponseWriter, r *http.Request) {
	startDate, endDate := parseDateRange(r)
	ctx := r.Context()

	// Query the database for completed bookings in range
	summary, err := h.bookingRepo.GetAccountingSummary(ctx, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get accounting summary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := AccountingSummaryResponse{
		TotalRevenue:          summary.TotalRevenue,
		TotalTherapistPayouts: summary.TotalTherapistPayouts,
		TotalPlatformProfit:   summary.TotalPlatformProfit,
		BookingCount:          summary.BookingCount,
		StartDate:             startDate.Format("2006-01-02"),
		EndDate:               endDate.Format("2006-01-02"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetDailyAccounting returns daily breakdown for charts (legacy, from bookings).
// GET /admin/reports/accounting/daily?start_date=...&end_date=...
func (h *ReportHandler) GetDailyAccounting(w http.ResponseWriter, r *http.Request) {
	startDate, endDate := parseDateRange(r)
	ctx := r.Context()

	// Query the database for daily breakdown
	dailyData, err := h.bookingRepo.GetDailyAccounting(ctx, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get daily accounting: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []DailyAccountingEntry
	for _, d := range dailyData {
		entries = append(entries, DailyAccountingEntry{
			Date:             d.Date.Format("2006-01-02"),
			Revenue:          d.Revenue,
			TherapistPayouts: d.TherapistPayouts,
			PlatformProfit:   d.PlatformProfit,
			BookingCount:     d.BookingCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"data":       entries,
	})
}

// GetLedgerSummary returns aggregated ledger data (Credits - Debits = Net Profit).
// GET /admin/reports/ledger/summary?start_date=...&end_date=...
func (h *ReportHandler) GetLedgerSummary(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	startDate, endDate := parseDateRange(r)
	ctx := r.Context()

	summary, err := h.ledgerRepo.GetSummary(ctx, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to get ledger summary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := LedgerSummaryResponse{
		TotalCredits: summary.TotalCredits,
		TotalDebits:  summary.TotalDebits,
		NetProfit:    summary.NetProfit,
		EntryCount:   summary.EntryCount,
		StartDate:    startDate.Format("2006-01-02"),
		EndDate:      endDate.Format("2006-01-02"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetLedgerTrend returns ledger data grouped by period (day, week, month, quarter, year).
// GET /admin/reports/ledger/trend?start_date=...&end_date=...&granularity=week
func (h *ReportHandler) GetLedgerTrend(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	startDate, endDate := parseDateRange(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	// Validate granularity
	validGranularities := map[string]bool{"day": true, "week": true, "month": true, "quarter": true, "year": true}
	if !validGranularities[granularity] {
		http.Error(w, "Invalid granularity. Allowed: day, week, month, quarter, year", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	summaries, err := h.ledgerRepo.GetSummaryByPeriod(ctx, startDate, endDate, granularity)
	if err != nil {
		http.Error(w, "Failed to get ledger trend: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []LedgerPeriodEntry
	for _, s := range summaries {
		entries = append(entries, LedgerPeriodEntry{
			PeriodStart:  s.PeriodStart.Format("2006-01-02"),
			TotalCredits: s.TotalCredits,
			TotalDebits:  s.TotalDebits,
			NetProfit:    s.NetProfit,
			EntryCount:   s.EntryCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"start_date":  startDate.Format("2006-01-02"),
		"end_date":    endDate.Format("2006-01-02"),
		"granularity": granularity,
		"data":        entries,
	})
}

// ListExpenses returns expense entries for a date range.
// GET /admin/reports/expenses?start_date=...&end_date=...
func (h *ReportHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	startDate, endDate := parseDateRange(r)
	ctx := r.Context()

	expenses, err := h.ledgerRepo.ListExpenses(ctx, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to list expenses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"data":       expenses,
	})
}

// CreateExpenseRequest is the request body for creating an expense.
type CreateExpenseRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"` // Defaults to "expense"
	EntryDate   string  `json:"entry_date"`
	ProofURL    *string `json:"proof_url,omitempty"` // Optional receipt/invoice URL
}

// CreateExpense adds a manual expense entry to the ledger.
// POST /admin/reports/expenses
func (h *ReportHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	var req CreateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	entryDate := time.Now()
	if req.EntryDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.EntryDate); err == nil {
			entryDate = parsed
		}
	}

	// Get actor ID from context (set by auth middleware)
	actorID, _ := middleware.GetUserID(r)

	category := repository.LedgerCategoryExpense
	if req.Category != "" && req.Category != "expense" {
		// Allow "adjustment" as well
		if req.Category == "adjustment" {
			category = repository.LedgerCategoryAdjustment
		}
	}

	err := h.ledgerRepo.InsertExpense(r.Context(), req.Amount, req.Description, category, entryDate, actorID, req.ProofURL)
	if err != nil {
		http.Error(w, "Failed to create expense: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// DeleteExpense removes an expense entry by ID.
// DELETE /admin/reports/expenses/{id}?reason=...
// Now implemented as Soft Delete (Void) for audit purposes.
func (h *ReportHandler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	if h.ledgerRepo == nil {
		http.Error(w, "Ledger not configured", http.StatusNotImplemented)
		return
	}

	// Extract ID from URL path
	// Assuming chi router: /expenses/{id}
	idStr := r.PathValue("id")
	// Fallback for older chi/routing versions or direct usage
	if idStr == "" {
		// Try to get from context if using go-chi standard middleware
		// But here we can't easily access chi context without import.
		// r.PathValue is Go 1.22+. If this project is older, we might need alternatives.
		// Looking at lines 324, it uses r.PathValue("id").
	}

	if idStr == "" {
		http.Error(w, "Missing expense ID", http.StatusBadRequest)
		return
	}

	entryID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid expense ID", http.StatusBadRequest)
		return
	}

	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "Manual deletion via admin dashboard"
	}

	if err := h.ledgerRepo.VoidEntry(r.Context(), entryID, reason); err != nil {
		http.Error(w, "Failed to void expense: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UploadExpenseReceipt uploads a receipt/invoice for expense documentation.
// POST /admin/reports/expenses/upload
// Returns the S3 URL of the uploaded file.
func (h *ReportHandler) UploadExpenseReceipt(w http.ResponseWriter, r *http.Request) {
	// Verify storage is configured
	if h.storageService == nil || !h.storageService.IsConfigured() {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Generate date-partitioned storage key: expenses/YYYY/MM/filename_timestamp.ext
	now := time.Now()
	prefix := fmt.Sprintf("expenses/%d/%02d", now.Year(), now.Month())
	key := h.storageService.GenerateKey(prefix, header.Filename)

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to storage
	fileURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		slog.Warn("storage upload error", "error", err)
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": fileURL,
	})
}
