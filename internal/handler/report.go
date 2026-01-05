package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ReportHandler handles accounting and reporting endpoints.
type ReportHandler struct {
	bookingRepo repository.BookingRepository
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(br repository.BookingRepository) *ReportHandler {
	return &ReportHandler{bookingRepo: br}
}

// AccountingSummaryResponse is the response for accounting summary.
type AccountingSummaryResponse struct {
	TotalRevenue           float64 `json:"total_revenue"`
	TotalTherapistPayouts  float64 `json:"total_therapist_payouts"`
	TotalPlatformProfit    float64 `json:"total_platform_profit"`
	BookingCount           int     `json:"booking_count"`
	StartDate              string  `json:"start_date"`
	EndDate                string  `json:"end_date"`
}

// DailyAccountingEntry represents a single day's accounting data.
type DailyAccountingEntry struct {
	Date                  string  `json:"date"`
	Revenue               float64 `json:"revenue"`
	TherapistPayouts      float64 `json:"therapist_payouts"`
	PlatformProfit        float64 `json:"platform_profit"`
	BookingCount          int     `json:"booking_count"`
}

// GetAccountingSummary returns aggregated accounting data for a date range.
// GET /admin/reports/accounting/summary?start_date=...&end_date=...
func (h *ReportHandler) GetAccountingSummary(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Default to last 30 days if not provided
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

// GetDailyAccounting returns daily breakdown for charts.
// GET /admin/reports/accounting/daily?start_date=...&end_date=...
func (h *ReportHandler) GetDailyAccounting(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Default to last 30 days if not provided
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = parsed.Add(24*time.Hour - time.Second)
		}
	}

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
