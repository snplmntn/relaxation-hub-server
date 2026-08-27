package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/xuri/excelize/v2"
)

type BookingReportScope struct {
	ClientID    *int64
	GeneratedAt time.Time
}

func BuildBookingReportWorkbook(rows []repository.BookingDetailsResult, scope BookingReportScope) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if scope.GeneratedAt.IsZero() {
		scope.GeneratedAt = time.Now()
	}

	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet("Bookings"); err != nil {
		return nil, err
	}

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	if err := writeBookingReportSummary(f, rows, scope, headerStyle); err != nil {
		return nil, err
	}
	if err := writeBookingReportRows(f, rows, headerStyle); err != nil {
		return nil, err
	}

	return workbookBytes(f)
}

func writeBookingReportSummary(f *excelize.File, rows []repository.BookingDetailsResult, scope BookingReportScope, headerStyle int) error {
	scopeLabel := "All clients"
	if scope.ClientID != nil {
		scopeLabel = fmt.Sprintf("Client #%d", *scope.ClientID)
	}

	totalAmount := 0.0
	statusCounts := make(map[string]int)
	for _, row := range rows {
		if row.Booking == nil {
			continue
		}
		statusCounts[row.Booking.Status]++
		if row.Booking.FinalTotal != nil {
			totalAmount += *row.Booking.FinalTotal
		}
	}

	if err := setCells(f, "Summary", map[string]any{
		"A1": "Booking Report",
		"A2": "Scope",
		"B2": scopeLabel,
		"A3": "Generated At",
		"B3": scope.GeneratedAt.Format(time.RFC3339),
		"A5": "Total Bookings",
		"B5": len(rows),
		"A6": "Total Final Amount",
		"B6": totalAmount,
		"A8": "Status",
		"B8": "Count",
	}); err != nil {
		return err
	}
	if err := f.SetCellStyle("Summary", "A1", "A1", headerStyle); err != nil {
		return err
	}
	if err := f.SetCellStyle("Summary", "A8", "B8", headerStyle); err != nil {
		return err
	}

	statuses := []string{"pending", "assigned", "on_the_way", "arrived", "in_progress", "completed", "cancelled", "no_show", "paid", "rescheduled"}
	written := make(map[string]bool)
	rowNumber := 9
	for _, status := range statuses {
		count := statusCounts[status]
		if count == 0 {
			continue
		}
		if err := writeRow(f, "Summary", rowNumber, []any{status, count}); err != nil {
			return err
		}
		written[status] = true
		rowNumber++
	}
	for status, count := range statusCounts {
		if written[status] {
			continue
		}
		if err := writeRow(f, "Summary", rowNumber, []any{status, count}); err != nil {
			return err
		}
		rowNumber++
	}
	return nil
}

func writeBookingReportRows(f *excelize.File, rows []repository.BookingDetailsResult, headerStyle int) error {
	headers := []any{
		"Booking ID",
		"Reference Code",
		"Client ID",
		"Client Name",
		"Client Phone",
		"Service",
		"Therapist",
		"Request?",
		"Status",
		"Scheduled Start",
		"Actual Start",
		"Actual End",
		"Duration Minutes",
		"Payment Method",
		"Raw Total",
		"Discount",
		"Final Total",
		"Promo Code",
		"Address",
		"Created At",
		"Cancellation Reason",
	}
	if err := writeRow(f, "Bookings", 1, headers); err != nil {
		return err
	}
	if err := f.SetCellStyle("Bookings", "A1", "U1", headerStyle); err != nil {
		return err
	}

	for i, result := range rows {
		if result.Booking == nil {
			continue
		}
		b := result.Booking
		values := []any{
			b.BookingID,
			stringPtrValue(b.ReferenceCode),
			b.ClientID,
			result.ClientName,
			result.ClientPhone,
			bookingReportServiceName(result),
			bookingReportTherapistName(result),
			yesNo(b.IsTherapistRequested),
			b.Status,
			timePtrValue(b.ScheduledStart),
			timePtrValue(b.ActualStart),
			timePtrValue(b.ActualEnd),
			b.DurationMinutes,
			b.PaymentMethod,
			floatPtrValue(b.RawTotal),
			floatPtrValue(b.Discount),
			floatPtrValue(b.FinalTotal),
			result.PromoCode,
			bookingReportAddress(result),
			b.CreatedAt,
			stringPtrValue(b.CancellationReason),
		}
		if err := writeRow(f, "Bookings", i+2, values); err != nil {
			return err
		}
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func bookingReportServiceName(result repository.BookingDetailsResult) string {
	if result.Service == nil {
		return ""
	}
	return result.Service.Name
}

func bookingReportTherapistName(result repository.BookingDetailsResult) string {
	value := strings.TrimSpace(result.TherapistName)
	if value == "" {
		return "Unassigned"
	}
	return value
}

func bookingReportAddress(result repository.BookingDetailsResult) string {
	if result.Address == nil {
		return ""
	}
	parts := []string{
		result.Address.Street,
		result.Address.Barangay,
		result.Address.City,
		result.Address.Province,
		result.Address.PostalCode,
		result.Address.Country,
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ", ")
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatPtrValue(value *float64) any {
	if value == nil {
		return ""
	}
	return *value
}

func timePtrValue(value *time.Time) any {
	if value == nil {
		return ""
	}
	return *value
}
