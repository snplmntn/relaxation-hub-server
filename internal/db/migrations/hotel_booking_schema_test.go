package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestHotelBookingAttributionMigrationAddsAndBackfillsHotelID(t *testing.T) {
	content, err := os.ReadFile("043_add_booking_partner_hotel.sql")
	if err != nil {
		t.Fatalf("read hotel attribution migration: %v", err)
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(string(content))), " ")

	for _, required := range []string{
		"add column if not exists partner_hotel_id bigint",
		"references public.partner_hotels(partner_hotel_id)",
		"alter table public.recurring_bookings",
		"update public.bookings as booking",
		"from public.partner_hotel_staff as staff",
		"where staff.user_id = booking.client_id",
		"having count(distinct hotel.partner_hotel_id) = 1",
		"create index if not exists idx_bookings_partner_hotel_schedule",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing hotel attribution migration containing %q", required)
		}
	}
}
