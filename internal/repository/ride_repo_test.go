package repository

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRideRepoCreate_PersistsScheduledFor(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewRideRepository(mockDB)
	bookingID := int64(321)
	distanceKm := 5.4
	scheduledFor := time.Date(2026, time.May, 10, 17, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	ride := &model.Ride{
		PassengerID:     42,
		BookingID:       &bookingID,
		RideType:        "return",
		PickupLat:       14.55,
		PickupLong:      121.02,
		PickupAddress:   "Client address",
		DropoffLat:      14.60,
		DropoffLong:     121.04,
		DropoffAddress:  "Branch address",
		DistanceKm:      &distanceKm,
		Status:          "pending",
		ScheduledFor:    &scheduledFor,
		PricingSnapshot: []byte(`{"final_fare":120}`),
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into rides") &&
			strings.Contains(lower, "scheduled_for") &&
			strings.Contains(lower, "pricing_snapshot")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 13 &&
			args[0] == ride.PassengerID &&
			args[1] == ride.BookingID &&
			args[2] == ride.RideType &&
			args[10] == ride.Status &&
			args[11] == ride.ScheduledFor &&
			bytes.Equal(args[12].([]byte), ride.PricingSnapshot)
	})).Return(row).Once()
	row.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 987
		*args.Get(1).(*time.Time) = createdAt
		*args.Get(2).(*time.Time) = updatedAt
	}).Return(nil).Once()

	err := repo.Create(context.Background(), ride)

	assert.NoError(t, err)
	assert.Equal(t, int64(987), ride.RideID)
	assert.Equal(t, createdAt, ride.CreatedAt)
	assert.Equal(t, updatedAt, ride.UpdatedAt)
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}
