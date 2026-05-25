package repository

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestRideRepoGetRidesByBookingIDScansRideType(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewRideRepository(mockDB)
	bookingID := int64(321)
	rideID := int64(987)
	passengerID := int64(42)
	distanceKm := 5.4
	createdAt := time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from rides") &&
			strings.Contains(lower, "booking_id = $1") &&
			strings.Contains(lower, "ride_type")
	}), []interface{}{bookingID}).Return(rows, nil).Once()
	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = rideID
		*args.Get(2).(*int64) = passengerID
		*args.Get(3).(**int64) = &bookingID
		*args.Get(4).(*string) = "outbound"
		*args.Get(5).(*float64) = 14.55
		*args.Get(6).(*float64) = 121.02
		*args.Get(7).(*string) = "Therapist pickup"
		*args.Get(8).(*float64) = 14.60
		*args.Get(9).(*float64) = 121.04
		*args.Get(10).(*string) = "Client dropoff"
		*args.Get(11).(**float64) = &distanceKm
		*args.Get(13).(*string) = "pending"
		*args.Get(14).(*time.Time) = createdAt
		*args.Get(19).(*string) = "Rider Name"
		*args.Get(20).(*string) = "09170000000"
		*args.Get(21).(*string) = "motorcycle"
		*args.Get(22).(*string) = "ABC123"
	}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	rides, err := repo.GetRidesByBookingID(context.Background(), bookingID)

	assert.NoError(t, err)
	if assert.Len(t, rides, 1) {
		assert.Equal(t, "outbound", rides[0].RideType)
		assert.Equal(t, "Rider Name", rides[0].RiderName)
	}
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestRideRepoGetByIDIncludesRiderDisplayFields(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewRideRepository(mockDB)
	rideID := int64(987)
	passengerID := int64(42)
	bookingID := int64(321)
	riderID := int64(654)
	distanceKm := 5.4
	createdAt := time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from rides r") &&
			strings.Contains(lower, "left join rider_profiles rp") &&
			strings.Contains(lower, "left join users u") &&
			strings.Contains(lower, "coalesce(u.full_name") &&
			strings.Contains(lower, "coalesce(rp.vehicle_type")
	}), []interface{}{rideID}).Return(row).Once()
	row.On("Scan",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = rideID
		*args.Get(1).(**int64) = &riderID
		*args.Get(2).(*int64) = passengerID
		*args.Get(3).(**int64) = &bookingID
		*args.Get(4).(*string) = "return"
		*args.Get(5).(*float64) = 14.55
		*args.Get(6).(*float64) = 121.02
		*args.Get(7).(*string) = "Client pickup"
		*args.Get(8).(*float64) = 14.60
		*args.Get(9).(*float64) = 121.04
		*args.Get(10).(*string) = "Branch dropoff"
		*args.Get(11).(**float64) = &distanceKm
		*args.Get(13).(*string) = "accepted"
		*args.Get(14).(*time.Time) = createdAt
		*args.Get(19).(*string) = "Rider Name"
		*args.Get(20).(*string) = "09170000000"
		*args.Get(21).(*string) = "motorcycle"
		*args.Get(22).(*string) = "ABC123"
	}).Return(nil).Once()

	ride, err := repo.GetByID(context.Background(), rideID)

	assert.NoError(t, err)
	if assert.NotNil(t, ride) {
		assert.Equal(t, "return", ride.RideType)
		assert.Equal(t, "Rider Name", ride.RiderName)
		assert.Equal(t, "motorcycle", ride.VehicleType)
	}
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestRideRepoUpdateRiderProfileRejectsUnknownColumn(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewRideRepository(mockDB)

	err := repo.UpdateRiderProfile(context.Background(), 33, map[string]interface{}{
		"vehicle_type = 'car', rating": 5,
	})
	if err == nil {
		t.Fatalf("expected invalid update field error")
	}
	mockDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
}

func TestRideRepoUpdateRiderProfileAllowsWhitelistedColumns(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewRideRepository(mockDB)
	branchID := int64(3)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "update rider_profiles set") &&
			strings.Contains(sql, "usual_branch_id = $1") &&
			strings.Contains(sql, "updated_at = now()") &&
			!strings.Contains(sql, "rating")
	}), []interface{}{branchID, int64(33)}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.UpdateRiderProfile(context.Background(), 33, map[string]interface{}{"usual_branch_id": branchID})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

func TestRideRepoCreateRiderProfileIsIdempotentByUser(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewRideRepository(mockDB)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		sql = strings.ToLower(sql)
		return strings.Contains(sql, "insert into rider_profiles") &&
			strings.Contains(sql, "on conflict (user_id) do update") &&
			strings.Contains(sql, "updated_at = now()")
	}), []interface{}{int64(12), "motorcycle", "ABC123"}).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()

	err := repo.CreateRiderProfile(context.Background(), 12, "motorcycle", "ABC123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	mockDB.AssertExpectations(t)
}

var _ = errors.Is
