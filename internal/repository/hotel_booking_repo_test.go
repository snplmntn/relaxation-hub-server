package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestHotelAnalyticsScopesBookingsByPartnerHotelID(t *testing.T) {
	database := new(MockDBTX)
	summary := new(MockRow)
	repo := NewHotelBookingRepository(database)

	assertDirectHotelScope := func(sql string) bool {
		normalized := strings.ToLower(sql)
		return strings.Contains(normalized, "partner_hotel_id=$1") &&
			!strings.Contains(normalized, "partner_hotel_staff")
	}

	database.On("QueryRow", mock.Anything, mock.MatchedBy(assertDirectHotelScope), mock.Anything).
		Return(summary).Once()
	summary.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*string) = "Hotel"
		}).Return(nil).Once()

	for range 4 {
		database.On("Query", mock.Anything, mock.MatchedBy(assertDirectHotelScope), mock.Anything).
			Return(emptyMockRows(), nil).Once()
	}

	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	until := from.AddDate(0, 0, 30)
	result, err := repo.Analytics(context.Background(), 4, from, until)
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}
	if result.HotelName != "Hotel" {
		t.Fatalf("expected Hotel, got %q", result.HotelName)
	}

	database.AssertExpectations(t)
	summary.AssertExpectations(t)
}
