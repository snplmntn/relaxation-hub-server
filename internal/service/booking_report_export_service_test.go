package service

import (
	"bytes"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/xuri/excelize/v2"
)

func TestBuildBookingReportWorkbookContainsSummaryAndRows(t *testing.T) {
	ref := "RH-100"
	total := 1200.0
	scheduled := time.Date(2026, 5, 23, 7, 0, 0, 0, time.UTC)
	created := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	clientID := int64(42)

	workbook, err := BuildBookingReportWorkbook([]repository.BookingDetailsResult{
		{
			Booking: &model.Booking{
				BookingID:       100,
				ReferenceCode:   &ref,
				ClientID:        clientID,
				Status:          model.BookingStatusCompleted,
				ScheduledStart:  &scheduled,
				DurationMinutes: 60,
				PaymentMethod:   "cash",
				FinalTotal:      &total,
				CreatedAt:       created,
			},
			Service:       &model.Service{Name: "Swedish Massage"},
			Address:       &model.Address{Street: "123 Main", City: "Manila", Country: "Philippines"},
			ClientName:    "Maria Client",
			ClientPhone:   "09171234567",
			TherapistName: "Anna Therapist",
		},
	}, BookingReportScope{
		ClientID:    &clientID,
		GeneratedAt: time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildBookingReportWorkbook returned error: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(workbook))
	if err != nil {
		t.Fatalf("failed to open workbook: %v", err)
	}
	defer f.Close()

	if value, _ := f.GetCellValue("Summary", "A1"); value != "Booking Report" {
		t.Fatalf("expected summary title, got %q", value)
	}
	if value, _ := f.GetCellValue("Summary", "B2"); value != "Client #42" {
		t.Fatalf("expected client scope, got %q", value)
	}
	if value, _ := f.GetCellValue("Summary", "B5"); value != "1" {
		t.Fatalf("expected total booking count, got %q", value)
	}
	if value, _ := f.GetCellValue("Bookings", "B2"); value != "RH-100" {
		t.Fatalf("expected reference code, got %q", value)
	}
	if value, _ := f.GetCellValue("Bookings", "F2"); value != "Swedish Massage" {
		t.Fatalf("expected service name, got %q", value)
	}
	if value, _ := f.GetCellValue("Bookings", "H2"); value != model.BookingStatusCompleted {
		t.Fatalf("expected completed status, got %q", value)
	}
}
