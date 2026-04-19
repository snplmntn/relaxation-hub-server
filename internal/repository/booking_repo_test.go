package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBookingRepoCreateTx_PersistsGroupFields(t *testing.T) {
	tx := new(MockTx)
	row := new(MockRow)
	repo := NewBookingRepository(new(MockDBTX))

	groupID := int64(77)
	serviceID := int64(12)
	promoID := int64(55)
	rawTotal := 150.0
	discount := 10.0
	finalTotal := 140.0
	referenceCode := "RH-123"
	now := time.Now().UTC()

	booking := &model.Booking{
		ClientID:        999,
		ServiceID:       &serviceID,
		PromoID:         &promoID,
		PaymentMethod:   "cash",
		DurationMinutes: 60,
		ScheduledStart:  &now,
		RawTotal:        &rawTotal,
		Discount:        &discount,
		FinalTotal:      &finalTotal,
		Status:          model.BookingStatusPending,
		ReferenceCode:   &referenceCode,
		GroupID:         &groupID,
		GuestName:       "Guest 1",
		SequenceNumber:  1,
		StartCondition:  "after_previous",
	}

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into bookings") &&
			strings.Contains(lower, "group_id") &&
			strings.Contains(lower, "guest_name") &&
			strings.Contains(lower, "sequence_number") &&
			strings.Contains(lower, "start_condition")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) >= 21 &&
			args[17] == booking.GroupID &&
			args[18] == booking.GuestName &&
			args[19] == booking.SequenceNumber &&
			args[20] == booking.StartCondition
	})).Return(row).Once()

	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 101
		*args.Get(1).(*time.Time) = now
		*args.Get(2).(*time.Time) = now
	}).Return(nil).Once()

	err := repo.CreateTx(context.Background(), tx, booking)
	assert.NoError(t, err)
	assert.Equal(t, int64(101), booking.BookingID)

	tx.AssertExpectations(t)
	row.AssertExpectations(t)
}
