package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRecurringBookingRepoCreatePersistsPartnerHotel(t *testing.T) {
	database := new(MockDBTX)
	row := new(MockRow)
	repo := NewRecurringBookingRepository(database)
	hotelID := int64(7)
	rec := &model.RecurringBooking{
		ClientID:       12,
		PartnerHotelID: &hotelID,
		Frequency:      "daily",
		IntervalValue:  1,
		TimeOfDay:      "09:00",
		StartDate:      time.Now(),
		Status:         "active",
	}

	database.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "partner_hotel_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 21 && args[1] == rec.PartnerHotelID
	})).Return(row).Once()
	row.On("Scan", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	require.NoError(t, repo.Create(context.Background(), rec))
	database.AssertExpectations(t)
}
